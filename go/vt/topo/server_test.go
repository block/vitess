/*
Copyright 2026 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package topo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactServerAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"host list", "host1:2379,host2:2379", "host1:2379,host2:2379"},
		{"empty", "", ""},
		{"dsn", "user:secret@tcp(db:3306)/topo", "<redacted>@tcp(db:3306)/topo"},
		{"password containing @", "user:se@cr@et@tcp(db:3306)/topo", "<redacted>@tcp(db:3306)/topo"},
		{"password containing /", "user:se/cret@tcp(db:3306)/topo", "<redacted>@tcp(db:3306)/topo"},
		{"user only", "user@tcp(db:3306)/topo", "<redacted>@tcp(db:3306)/topo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactServerAddress(tt.addr)
			require.Equal(t, tt.want, got)
			require.NotContains(t, got, "secret")
			require.NotContains(t, got, "cr@et")
		})
	}
}
