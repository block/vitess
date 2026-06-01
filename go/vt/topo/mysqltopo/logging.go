// Copyright 2026 The Vitess Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package mysqltopo

import (
	"fmt"

	"vitess.io/vitess/go/vt/log"
)

// release-23.0's vt/log exposed printf-style helpers (Infof/Warningf/Errorf),
// release-24.0 rewrote vt/log around slog. Wrap the new structured API so the
// rest of this package stays close to its release-23.0 form. Convert to slog
// attributes when revisiting these call sites.
var (
	logInfof    = func(format string, args ...any) { log.Info(fmt.Sprintf(format, args...)) }
	logWarningf = func(format string, args ...any) { log.Warn(fmt.Sprintf(format, args...)) }
	logErrorf   = func(format string, args ...any) { log.Error(fmt.Sprintf(format, args...)) }
)
