package config

import (
	"os"
	"os/user"
	"strings"

	"github.com/adrg/xdg"
	"gopkg.in/ini.v1"
)

var configFilePath string
var cfg *ini.File

type IniFile struct {
	*General
	*Keymap
	*Ui
	*Colors
}

type General struct {
	DownloadPath        string
	PreviewPath         string
	CmdPrefix           string
	ShowCommand         string
	EnableNotifications bool
	UseTerminalBell     bool
	NotificationTimeout int64
	BacklogMsgQuantity  int
}

type Keymap struct {
	SwitchPanels    string
	FocusMessages   string
	FocusInput      string
	FocusChats      string
	Copyuser        string
	Pasteuser       string
	CommandBacklog  string
	CommandRead     string
	CommandConnect  string
	CommandQuit     string
	CommandHelp     string
	MessageDownload string
	MessageOpen     string
	MessageShow     string
	MessageUrl      string
	MessageInfo     string
	MessageRevoke   string
}

type Ui struct {
	ChatSidebarWidth int
}

type Colors struct {
	Background      string
	Text            string
	ForwardedText   string
	ListHeader      string
	ListContact     string
	ListGroup       string
	ChatContact     string
	ChatMe          string
	Borders         string
	InputBackground string
	InputText       string
	UnreadCount     string
	Positive        string
	Negative        string
	// AutoContrast raises colors that would be unreadable on the configured
	// background; set it to false to keep the exact configured values.
	AutoContrast bool
	// Theme selects a built-in palette by name. Individual color keys in this
	// section still win over the preset, so a theme can be adjusted in place.
	// Empty means "only use the keys above".
	Theme string
}

// ColorKeysSet records which color keys the config file actually contains, so
// a `theme` preset can fill in the rest without silently overwriting values the
// user wrote by hand. Filled by InitConfig.
var ColorKeysSet = map[string]bool{}

var Config = IniFile{
	&General{
		DownloadPath:        GetHomeDir() + "Downloads",
		PreviewPath:         GetHomeDir() + "Downloads",
		CmdPrefix:           "/",
		ShowCommand:         "jp2a --color",
		EnableNotifications: false,
		UseTerminalBell:     false,
		NotificationTimeout: 60,
		BacklogMsgQuantity:  10,
	},
	&Keymap{
		SwitchPanels:    "Tab",
		FocusMessages:   "Ctrl+w",
		FocusInput:      "Ctrl+Space",
		FocusChats:      "Ctrl+e",
		CommandBacklog:  "Ctrl+b",
		CommandRead:     "Ctrl+n",
		Copyuser:        "Ctrl+c",
		Pasteuser:       "Ctrl+v",
		CommandConnect:  "Ctrl+r",
		CommandQuit:     "Ctrl+q",
		CommandHelp:     "Ctrl+?",
		MessageDownload: "d",
		MessageInfo:     "i",
		MessageOpen:     "o",
		MessageUrl:      "u",
		MessageRevoke:   "r",
		MessageShow:     "s",
	},
	&Ui{
		ChatSidebarWidth: 30,
	},
	&Colors{
		// Names, not hex: go-ini treats a leading "#" as a comment, so hex
		// defaults would be written with awkward quoting.
		Background:      "black",
		Text:            "lightgray",
		ForwardedText:   "mediumpurple",
		ListHeader:      "burlywood",
		ListContact:     "darkseagreen",
		ListGroup:       "cornflowerblue",
		ChatContact:     "darkseagreen",
		ChatMe:          "cornflowerblue",
		Borders:         "slategray",
		InputBackground: "darkslategray",
		InputText:       "whitesmoke",
		UnreadCount:     "burlywood",
		Positive:        "darkseagreen",
		Negative:        "lightcoral",
		AutoContrast:    true,
		// The preset matching the defaults above. Presets only fill in keys the
		// config file does not set, so naming one here does not override an
		// existing config's own values.
		Theme: "dark",
	},
}

func InitConfig() error {
	var err error
	if configFilePath, err = xdg.ConfigFile("whatscli/whatscli.config"); err == nil {
		// add any new values
		var cfg *ini.File
		if cfg, err = ini.Load(configFilePath); err == nil {
			cfg.NameMapper = ini.TitleUnderscore
			cfg.ValueMapper = os.ExpandEnv
			if section, err := cfg.GetSection("general"); err == nil {
				section.MapTo(&Config.General)
			}
			if section, err := cfg.GetSection("keymap"); err == nil {
				section.MapTo(&Config.Keymap)
			}
			if section, err := cfg.GetSection("ui"); err == nil {
				section.MapTo(&Config.Ui)
			}
			if section, err := cfg.GetSection("colors"); err == nil {
				section.MapTo(&Config.Colors)
				ColorKeysSet = colorKeys(section)
			}
		} else {
			cfg = ini.Empty()
			cfg.NameMapper = ini.TitleUnderscore
			cfg.ValueMapper = os.ExpandEnv
			if err = ini.ReflectFromWithMapper(cfg, &Config, ini.TitleUnderscore); err == nil {
				err = cfg.SaveTo(configFilePath)
			}
		}
	}
	return err
}

// colorKeys returns the lowercased names of the keys present in the [colors]
// section. MapTo cannot tell "absent" from "set to the default value", so the
// theme presets need this to know which entries the user actually wrote.
func colorKeys(section *ini.Section) map[string]bool {
	keys := make(map[string]bool, len(section.Keys()))
	for _, key := range section.KeyStrings() {
		keys[strings.ToLower(key)] = true
	}
	return keys
}

func GetConfigFilePath() string {
	return configFilePath
}

func GetSessionFilePath() string {
	if sessionFilePath, err := xdg.ConfigFile("whatscli/session"); err == nil {
		return sessionFilePath
	}
	return GetHomeDir() + ".whatscli.session"
}

// gets the OS home dir with a path separator at the end
func GetHomeDir() string {
	usr, err := user.Current()
	if err == nil {
		return usr.HomeDir + string(os.PathSeparator)
	}
	// Fallback to environment variable
	home := os.Getenv("HOME")
	if home != "" {
		return home + string(os.PathSeparator)
	}
	return "." + string(os.PathSeparator)
}
