package applications

import (
	"OpenLinkHub/src/common"
	"errors"
	"path/filepath"
)

// Add stores an application that is not in the bundled catalog, such as a game linked to an
// imported iCUE profile, and makes it available right away.
func Add(application Application) error {
	if !common.AlphanumericDashRegex.MatchString(application.Id) || application.Name == "" {
		return errors.New("invalid application")
	}
	if !application.Manual && len(application.Executables) == 0 {
		return errors.New("application needs an executable")
	}
	if err := common.SaveJsonData(filepath.Join(location, application.Id+".json"), application); err != nil {
		return err
	}
	applications[application.Id] = application
	return nil
}
