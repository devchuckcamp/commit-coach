package config

import "errors"

// ErrSetupRequired indicates required configuration is missing and should be
// collected interactively (e.g., provider/model/API key).
var ErrSetupRequired = errors.New("setup required")
