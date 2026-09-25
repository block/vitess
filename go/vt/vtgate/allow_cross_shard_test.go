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

package vtgate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"vitess.io/vitess/go/streamlog"
	vtgatepb "vitess.io/vitess/go/vt/proto/vtgate"
	"vitess.io/vitess/go/vt/sqlparser"
	econtext "vitess.io/vitess/go/vt/vtgate/executorcontext"
	"vitess.io/vitess/go/vt/vtgate/logstats"
)

const allowCrossShardQuery = "select /*vt+ ALLOW_CROSS_SHARD */ id from user"

// TestPreparedPlanCacheHitReturnsNoStatement pins the premise behind
// resolveAllowCrossShard: on a prepared-plan cache hit, fetchOrCreatePlan
// serves the plan straight out of the cache and never parses, so it returns a
// nil statement. Reading a query directive off that statement therefore yields
// false on every execution after the first, which is why the directive has to
// be recovered from the SQL text.
func TestPreparedPlanCacheHitReturnsNoStatement(t *testing.T) {
	executor, _, _, _, ctx := createExecutorEnv(t)

	newStats := func() *logstats.LogStats {
		return logstats.NewLogStats(ctx, "test", allowCrossShardQuery, "", nil, streamlog.GetQueryLogConfig())
	}
	sess := econtext.NewSafeSession(&vtgatepb.Session{TargetString: KsTestSharded})

	_, _, stmt, err := executor.fetchOrCreatePlan(ctx, sess, allowCrossShardQuery, nil, false, true, newStats(), true)
	require.NoError(t, err)
	require.NotNil(t, stmt, "the first (cache-miss) execution must parse the query")

	_, _, stmt, err = executor.fetchOrCreatePlan(ctx, sess, allowCrossShardQuery, nil, false, true, newStats(), true)
	require.NoError(t, err)
	require.Nil(t, stmt, "a prepared-plan cache hit must not re-parse the query")
}

// TestResolveAllowCrossShard verifies that the ALLOW_CROSS_SHARD directive is
// still honoured when no parsed statement is available, which is exactly the
// state a prepared-plan cache hit leaves us in. Without the SQL-text fallback
// the directive works on the first execution of a prepared statement and then
// silently stops, so the query starts failing the SINGLE-mode cross-shard
// check it was meant to bypass.
func TestResolveAllowCrossShard(t *testing.T) {
	executor, _, _, _, _ := createExecutorEnv(t)

	withDirective, err := executor.env.Parser().Parse(allowCrossShardQuery)
	require.NoError(t, err)
	withoutDirective, err := executor.env.Parser().Parse("select id from user")
	require.NoError(t, err)

	tests := []struct {
		name string
		sql  string
		stmt sqlparser.Statement
		want bool
	}{
		{"parsed statement carrying the directive", allowCrossShardQuery, withDirective, true},
		{"parsed statement without the directive", "select id from user", withoutDirective, false},
		{"cache hit, directive in the SQL text", allowCrossShardQuery, nil, true},
		{"cache hit, no directive", "select id from user", nil, false},
		{"cache hit, directive named only inside a literal", "insert into user(name) values ('ALLOW_CROSS_SHARD')", nil, false},
		{"cache hit, unparseable SQL", "this is not sql ALLOW_CROSS_SHARD", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, executor.resolveAllowCrossShard(tt.sql, tt.stmt))
		})
	}
}
