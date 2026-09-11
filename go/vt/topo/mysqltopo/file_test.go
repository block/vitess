/*
Copyright 2025 The Vitess Authors.

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

package mysqltopo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestListKeepsPrefixSiblings pins the other half of the directory/prefix
// split: topo.Conn.List takes a path prefix, not a directory, so a prefix
// sibling ("keyspaces/foobar" for "keyspaces/foo") must still be returned.
// This is what go/vt/topo/test/file.go:checkList() expects.
func TestListKeepsPrefixSiblings(t *testing.T) {
	server, _, cleanup := createTestServer(t, "")
	defer cleanup()

	ctx := context.Background()

	_, err := server.Create(ctx, "keyspaces/foo/Keyspace", []byte("foo"))
	require.NoError(t, err)
	_, err = server.Create(ctx, "keyspaces/foobar/Keyspace", []byte("foobar"))
	require.NoError(t, err)

	kvs, err := server.List(ctx, "keyspaces/foo")
	require.NoError(t, err)

	paths := make([]string, 0, len(kvs))
	for _, kv := range kvs {
		paths = append(paths, string(kv.Key))
	}
	require.ElementsMatch(t, []string{
		"/test/keyspaces/foo/Keyspace",
		"/test/keyspaces/foobar/Keyspace",
	}, paths, "List must keep prefix semantics")
}
