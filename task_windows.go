package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

const taskName = "OpenLinkHub"

// The task runs in the user's session, because shortcuts, macros and the microphone mute need
// the interactive desktop, and with highest privileges, because the CPU sensor driver requires
// administrator rights. conhost --headless keeps the console window from appearing at logon.
const taskTemplate = `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Controls Corsair fans, pumps, lighting and peripherals.</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>%[1]s</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>%[1]s</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>4</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>999</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>%[2]s</Command>
      <Arguments>--headless "%[3]s"</Arguments>
      <WorkingDirectory>%[4]s</WorkingDirectory>
    </Exec>
  </Actions>
</Task>
`

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func installTask(exe string) error {
	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("unable to determine the current user: %w", err)
	}
	conhost := filepath.Join(os.Getenv("SystemRoot"), "System32", "conhost.exe")
	definition := fmt.Sprintf(taskTemplate, xmlEscape(u.Username), xmlEscape(conhost), xmlEscape(exe), xmlEscape(filepath.Dir(exe)))

	file, err := os.CreateTemp("", "openlinkhub-task-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())

	// schtasks only accepts the UTF-16 encoding the XML header declares.
	encoded := utf16.Encode([]rune(definition))
	buf := make([]byte, 2, 2+2*len(encoded))
	buf[0], buf[1] = 0xFF, 0xFE
	for _, r := range encoded {
		buf = append(buf, byte(r), byte(r>>8))
	}
	if _, err = file.Write(buf); err != nil {
		file.Close()
		return err
	}
	file.Close()

	out, err := exec.Command("schtasks", "/Create", "/TN", taskName, "/XML", file.Name(), "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to create the logon task (run as administrator): %s", strings.TrimSpace(string(out)))
	}
	fmt.Println("OpenLinkHub will start at logon. Start it now with: schtasks /Run /TN OpenLinkHub")
	return nil
}

func uninstallTask() error {
	out, err := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to remove the logon task: %s", strings.TrimSpace(string(out)))
	}
	fmt.Println("Logon task removed.")
	return nil
}
