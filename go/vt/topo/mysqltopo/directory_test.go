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

	"vitess.io/vitess/go/vt/topo"
)

// TestListDirIgnoresPrefixSiblings verifies that ListDir only reports nodes
// contained in the directory, not nodes whose path merely starts with it.
// The built-in "cells" / "cells_aliases" pair is the case that bites in
// production: a raw prefix scan of "cells" also matches "cells_aliases/...",
// and the leftover "_aliases" becomes a phantom cell that every
// cell-enumerating caller (e.g. GetKnownCells, and through it
// RebuildSrvVSchema) then tries to read.
func TestListDirIgnoresPrefixSiblings(t *testing.T) {
	server, _, cleanup := createTestServer(t, "")
	defer cleanup()

	ctx := context.Background()

	_, err := server.Create(ctx, "cells/zone1/CellInfo", []byte("cell"))
	require.NoError(t, err)
	_, err = server.Create(ctx, "cells_aliases/region1/CellsAlias", []byte("alias"))
	require.NoError(t, err)

	entries, err := server.ListDir(ctx, "cells", false)
	require.NoError(t, err)
	require.Equal(t, []topo.DirEntry{{Name: "zone1"}}, entries,
		"ListDir(cells) must not report a child derived from cells_aliases")

	entries, err = server.ListDir(ctx, "cells_aliases", false)
	require.NoError(t, err)
	require.Equal(t, []topo.DirEntry{{Name: "region1"}}, entries)
}

// TestListDirIgnoresPrefixSiblingKeyspaces verifies the same containment rule
// for user-chosen names: keyspace "foobar" starts with "foo", so a raw prefix
// scan lists foobar's children under foo.
func TestListDirIgnoresPrefixSiblingKeyspaces(t *testing.T) {
	server, _, cleanup := createTestServer(t, "")
	defer cleanup()

	ctx := context.Background()

	_, err := server.Create(ctx, "keyspaces/foo/Keyspace", []byte("foo"))
	require.NoError(t, err)
	_, err = server.Create(ctx, "keyspaces/foobar/shards/0/Shard", []byte("foobar shard"))
	require.NoError(t, err)

	entries, err := server.ListDir(ctx, "keyspaces/foo", false)
	require.NoError(t, err)
	require.Equal(t, []topo.DirEntry{{Name: "Keyspace"}}, entries,
		"ListDir(keyspaces/foo) must not leak foobar's children")

	// Both keyspaces are still children of the parent directory.
	entries, err = server.ListDir(ctx, "keyspaces", false)
	require.NoError(t, err)
	require.Equal(t, []topo.DirEntry{{Name: "foo"}, {Name: "foobar"}}, entries)
}

// TestListDirIgnoresPrefixSiblingLocks verifies the containment rule for the
// topo_locks half of ListDir: a lock held on a prefix sibling must not show up
// as an ephemeral child of the directory being listed.
func TestListDirIgnoresPrefixSiblingLocks(t *testing.T) {
	server, _, cleanup := createTestServer(t, "")
	defer cleanup()

	ctx := context.Background()

	_, err := server.Create(ctx, "keyspaces/foo/Keyspace", []byte("foo"))
	require.NoError(t, err)
	_, err = server.Create(ctx, "keyspaces/foobar/Keyspace", []byte("foobar"))
	require.NoError(t, err)

	// The lock row's path is exactly /test/keyspaces/foobar, which a raw
	// prefix scan of /test/keyspaces/foo turns into a child named "bar".
	lock, err := server.Lock(ctx, "keyspaces/foobar", "holder")
	require.NoError(t, err)
	defer lock.Unlock(ctx)

	entries, err := server.ListDir(ctx, "keyspaces/foo", false)
	require.NoError(t, err)
	require.Equal(t, []topo.DirEntry{{Name: "Keyspace"}}, entries,
		"ListDir(keyspaces/foo) must not report a child derived from the foobar lock")
}
