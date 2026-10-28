package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// colorKeyNames lists the canonical keys of the [colors] section (the `theme`
// and `auto_contrast` keys included). SaveTheme uses it to strip
// preset-covered overrides — it never deletes `theme` (it writes it) or
// `auto_contrast` (a user preference, not a palette entry). Keep in sync with
// the Colors struct — colorNamesMatchStruct asserts they agree.
var colorKeyNames = map[string]bool{
	"theme": true, "auto_contrast": true, "background": true, "text": true,
	"forwarded_text": true, "input_background": true, "input_text": true,
	"borders": true, "list_header": true, "list_contact": true,
	"list_group": true, "chat_contact": true, "chat_me": true,
	"unread_count": true, "positive": true, "negative": true,
}

func GetConfigFilePath() string {
	return configFilePath
}

// themesDirOverride is a test hook: when non-empty, GetThemesDir returns it.
var themesDirOverride string

// GetThemesDir returns the directory custom theme files are read from
// (themes/ next to the config file), or "" when no config path is known
// (tests without a config file). Custom theme INIs use the same [colors]
// keys as the main config; see findTheme's file fallback.
func GetThemesDir() string {
	if themesDirOverride != "" {
		return themesDirOverride
	}
	if configFilePath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(configFilePath), "themes")
}

// SetThemesDirForTest points GetThemesDir at dir and returns a restore func.
func SetThemesDirForTest(dir string) func() {
	prev := themesDirOverride
	themesDirOverride = dir
	return func() { themesDirOverride = prev }
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

// SaveTheme persists a theme choice to the config file: it sets the `theme`
// key in [colors] and, for a preset pick, removes the color-key overrides the
// file previously contained (a preset that applies "as written" is what the
// picker promised; keeping the keys would silently re-impose them at the next
// start). In the "custom" state (the empty name) nothing is deleted — the
// file's own keys stay — because a custom palette cannot be serialized as a
// preset until custom theme files exist (plan item D2).
//
// The original file is copied to <config>.bak before the first write-back,
// so a bad save is always one `mv` away from being undone.
func SaveTheme(path, name string, removeOverrides bool) error {
	if path == "" {
		return errors.New("no config file path recorded")
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	cfg, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, path)
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	section, err := cfg.GetSection("colors")
	if err != nil {
		section, err = cfg.NewSection("colors")
		if err != nil {
			return fmt.Errorf("create [colors]: %w", err)
		}
	}
	if removeOverrides {
		for _, key := range section.KeyStrings() {
			lower := strings.ToLower(key)
			if lower == "theme" || lower == "auto_contrast" {
				continue
			}
			if colorKeyNames[lower] {
				section.DeleteKey(key)
			}
		}
	}
	section.Key("theme").SetValue(name)
	// First write-back only: keep the pristine copy even across further saves.
	bak := path + ".bak"
	if _, statErr := os.Stat(bak); os.IsNotExist(statErr) {
		if err := os.WriteFile(bak, original, 0o600); err != nil {
			return fmt.Errorf("write backup: %w", err)
		}
	}
	if err := cfg.SaveTo(path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
