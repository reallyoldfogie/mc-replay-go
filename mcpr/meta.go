package mcpr

// CurrentFileFormatVersion is the latest ReplayMod MCPR format supported by this package.
const CurrentFileFormatVersion = 14

// Meta describes the metaData.json fields written alongside the packet stream.
// Only a subset is required by ReplayMod; optional fields are emitted when set.
type Meta struct {
    Singleplayer      bool     `json:"singleplayer"` // Always include, even if false
    ServerName        string   `json:"serverName,omitempty"`
    CustomServerName  string   `json:"customServerName,omitempty"`
    Duration          int      `json:"duration,omitempty"`          // milliseconds
    Date              int64    `json:"date,omitempty"`              // unix ms
    MCVersion         string   `json:"mcversion,omitempty"`
    FileFormat        string   `json:"fileFormat,omitempty"`
    FileFormatVersion int      `json:"fileFormatVersion,omitempty"`
    Protocol          int      `json:"protocol,omitempty"`          // MC network protocol id
    Generator         string   `json:"generator,omitempty"`
    SelfID            int      `json:"selfId,omitempty"`
    Players           []string `json:"players,omitempty"`
}

