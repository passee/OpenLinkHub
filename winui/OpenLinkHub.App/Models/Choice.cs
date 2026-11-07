namespace OpenLinkHub.App.Models;

public sealed record Choice(string Value, string Label)
{
    public override string ToString() => Label;
}
