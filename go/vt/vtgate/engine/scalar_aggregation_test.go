/*
Copyright 2022 The Vitess Authors.

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

package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"vitess.io/vitess/go/mysql/collations"
	"vitess.io/vitess/go/sqltypes"
	"vitess.io/vitess/go/test/utils"
	querypb "vitess.io/vitess/go/vt/proto/query"
	"vitess.io/vitess/go/vt/sqlparser"
	. "vitess.io/vitess/go/vt/vtgate/engine/opcode"
)

func TestEmptyRows(outer *testing.T) {
	testCases := []struct {
		opcode      AggregateOpcode
		origOpcode  AggregateOpcode
		expectedVal string
		expectedTyp string
		// notNull: the result column is NOT NULL. The input column is NOT
		// NULL in every case, and only a count keeps the flag: every other
		// aggregate is NULL over this empty input.
		notNull bool
	}{{
		opcode:      AggregateCountDistinct,
		expectedVal: "0",
		expectedTyp: "int64",
		notNull:     true,
	}, {
		opcode:      AggregateCount,
		expectedVal: "0",
		expectedTyp: "int64",
		notNull:     true,
	}, {
		opcode:      AggregateSumDistinct,
		expectedVal: "null",
		expectedTyp: "decimal",
	}, {
		opcode:      AggregateSum,
		expectedVal: "null",
		expectedTyp: "decimal",
	}, {
		opcode:      AggregateSum,
		expectedVal: "0",
		expectedTyp: "int64",
		origOpcode:  AggregateCount,
		notNull:     true,
	}, {
		opcode:      AggregateMax,
		expectedVal: "null",
		expectedTyp: "int64",
	}, {
		opcode:      AggregateMin,
		expectedVal: "null",
		expectedTyp: "int64",
	}}

	for _, test := range testCases {
		outer.Run(test.opcode.String(), func(t *testing.T) {
			fp := &fakePrimitive{
				results: []*sqltypes.Result{sqltypes.MakeTestResult(
					notNullFields([]int{0}, sqltypes.MakeTestFields(
						test.opcode.String(),
						"int64",
					)),
					// Empty input table
				)},
			}

			oa := &ScalarAggregate{
				Aggregates: []*AggregateParams{{
					Opcode:     test.opcode,
					Col:        0,
					Alias:      test.opcode.String(),
					OrigOpcode: test.origOpcode,
				}},
				Input: fp,
			}

			result, err := oa.TryExecute(context.Background(), &noopVCursor{}, nil, false)
			assert.NoError(t, err)

			wantFields := sqltypes.MakeTestFields(
				test.opcode.String(),
				test.expectedTyp,
			)
			if test.notNull {
				notNullFields([]int{0}, wantFields)
			}
			wantResult := sqltypes.MakeTestResult(wantFields, test.expectedVal)
			utils.MustMatch(t, wantResult, result)
		})
	}
}

func TestScalarAggregateStreamExecute(t *testing.T) {
	fields := sqltypes.MakeTestFields(
		"col|weight_string(col)",
		"uint64|varbinary",
	)
	fp := &fakePrimitive{
		allResultsInOneCall: true,
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(fields,
				"1|null",
			), sqltypes.MakeTestResult(fields,
				"3|null",
			),
		},
	}

	oa := &ScalarAggregate{
		Aggregates: []*AggregateParams{{
			Opcode: AggregateSum,
			Col:    0,
		}},
		Input:               fp,
		TruncateColumnCount: 1,
	}

	var results []*sqltypes.Result
	err := oa.TryStreamExecute(context.Background(), &noopVCursor{}, nil, true, func(qr *sqltypes.Result) error {
		results = append(results, qr)
		return nil
	})
	assert.NoError(t, err)
	// one for the fields, and one for the actual aggregation result
	require.EqualValues(t, 2, len(results), "number of results")

	got := fmt.Sprintf("%v", results[1].Rows)
	assert.Equal(t, "[[DECIMAL(4)]]", got)
}

// TestScalarAggregateExecuteTruncate checks if truncate works
func TestScalarAggregateExecuteTruncate(t *testing.T) {
	fields := sqltypes.MakeTestFields(
		"col|weight_string(col)",
		"uint64|varbinary",
	)

	fp := &fakePrimitive{
		allResultsInOneCall: true,
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(fields,
				"1|null", "3|null",
			),
		},
	}

	oa := &ScalarAggregate{
		Aggregates: []*AggregateParams{{
			Opcode: AggregateSum,
			Col:    0,
		}},
		Input:               fp,
		TruncateColumnCount: 1,
	}

	qr, err := oa.TryExecute(context.Background(), &noopVCursor{}, nil, true)
	assert.NoError(t, err)
	assert.Equal(t, "[[DECIMAL(4)]]", fmt.Sprintf("%v", qr.Rows))
}

// TestScalarGroupConcatWithAggrOnEngine tests group_concat with full aggregation on engine.
func TestScalarGroupConcatWithAggrOnEngine(t *testing.T) {
	fields := sqltypes.MakeTestFields(
		"c2",
		"varchar",
	)

	varbinaryFields := sqltypes.MakeTestFields(
		"c2",
		"varbinary",
	)

	textOutFields := sqltypes.MakeTestFields(
		"group_concat(c2)",
		"text",
	)

	tcases := []struct {
		name        string
		inputResult *sqltypes.Result
		expResult   *sqltypes.Result
	}{{
		name: "ending with null",
		inputResult: sqltypes.MakeTestResult(fields,
			"a", "a", "b", "null", "null"),
		expResult: sqltypes.MakeTestResult(textOutFields,
			`a,a,b`),
	}, {
		name:        "empty result",
		inputResult: sqltypes.MakeTestResult(fields),
		expResult: sqltypes.MakeTestResult(textOutFields,
			`null`),
	}, {
		name: "only null value for concat",
		inputResult: sqltypes.MakeTestResult(fields,
			"null", "null", "null"),
		expResult: sqltypes.MakeTestResult(textOutFields,
			`null`),
	}, {
		name: "empty string value for concat",
		inputResult: sqltypes.MakeTestResult(fields,
			"", "", ""),
		expResult: sqltypes.MakeTestResult(textOutFields,
			`,,`),
	}, {
		name: "varbinary column",
		inputResult: sqltypes.MakeTestResult(varbinaryFields,
			"foo", "null", "bar"),
		expResult: sqltypes.MakeTestResult(sqltypes.MakeTestFields(
			"group_concat(c2)",
			"blob",
		),
			`foo,bar`),
	}}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			fp := &fakePrimitive{results: []*sqltypes.Result{tcase.inputResult}}
			oa := &ScalarAggregate{
				Aggregates: []*AggregateParams{{
					Opcode: AggregateGroupConcat,
					Col:    0,
					Alias:  "group_concat(c2)",
					Func:   &sqlparser.GroupConcatExpr{Separator: ","},
				}},
				Input: fp,
			}
			qr, err := oa.TryExecute(context.Background(), &noopVCursor{}, nil, false)
			require.NoError(t, err)
			utils.MustMatch(t, tcase.expResult, qr)

			fp.rewind()
			results := &sqltypes.Result{}
			err = oa.TryStreamExecute(context.Background(), &noopVCursor{}, nil, true, func(qr *sqltypes.Result) error {
				if qr.Fields != nil {
					results.Fields = qr.Fields
				}
				results.Rows = append(results.Rows, qr.Rows...)
				return nil
			})
			require.NoError(t, err)
			utils.MustMatch(t, tcase.expResult, results)
		})
	}
}

// TestScalarDistinctAggr tests distinct aggregation on engine.
func TestScalarDistinctAggrOnEngine(t *testing.T) {
	fields := sqltypes.MakeTestFields(
		"value|value",
		"int64|int64",
	)

	fp := &fakePrimitive{results: []*sqltypes.Result{sqltypes.MakeTestResult(
		fields,
		"100|100",
		"200|200",
		"200|200",
		"400|400",
		"400|400",
		"600|600",
	)}}

	oa := &ScalarAggregate{
		Aggregates: []*AggregateParams{
			NewAggregateParam(AggregateCountDistinct, 0, nil, "count(distinct value)", collations.MySQL8()),
			NewAggregateParam(AggregateSumDistinct, 1, nil, "sum(distinct value)", collations.MySQL8()),
		},
		Input: fp,
	}
	qr, err := oa.TryExecute(context.Background(), &noopVCursor{}, nil, false)
	require.NoError(t, err)
	require.Equal(t, `[[INT64(4) DECIMAL(1300)]]`, fmt.Sprintf("%v", qr.Rows))

	fp.rewind()
	results := &sqltypes.Result{}
	err = oa.TryStreamExecute(context.Background(), &noopVCursor{}, nil, true, func(qr *sqltypes.Result) error {
		if qr.Fields != nil {
			results.Fields = qr.Fields
		}
		results.Rows = append(results.Rows, qr.Rows...)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, `[[INT64(4) DECIMAL(1300)]]`, fmt.Sprintf("%v", results.Rows))
}

func TestScalarDistinctPushedDown(t *testing.T) {
	fields := sqltypes.MakeTestFields(
		"count(distinct value)|sum(distinct value)",
		"int64|decimal",
	)

	fp := &fakePrimitive{results: []*sqltypes.Result{sqltypes.MakeTestResult(
		fields,
		"2|200",
		"6|400",
		"3|700",
		"1|10",
		"7|30",
		"8|90",
	)}}

	countAggr := NewAggregateParam(AggregateSum, 0, nil, "count(distinct value)", collations.MySQL8())
	countAggr.OrigOpcode = AggregateCountDistinct
	sumAggr := NewAggregateParam(AggregateSum, 1, nil, "sum(distinct value)", collations.MySQL8())
	sumAggr.OrigOpcode = AggregateSumDistinct
	oa := &ScalarAggregate{
		Aggregates: []*AggregateParams{
			countAggr,
			sumAggr,
		},
		Input: fp,
	}
	qr, err := oa.TryExecute(context.Background(), &noopVCursor{}, nil, false)
	require.NoError(t, err)
	require.Equal(t, `[[INT64(27) DECIMAL(1430)]]`, fmt.Sprintf("%v", qr.Rows))

	fp.rewind()
	results := &sqltypes.Result{}
	err = oa.TryStreamExecute(context.Background(), &noopVCursor{}, nil, true, func(qr *sqltypes.Result) error {
		if qr.Fields != nil {
			results.Fields = qr.Fields
		}
		results.Rows = append(results.Rows, qr.Rows...)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, `[[INT64(27) DECIMAL(1430)]]`, fmt.Sprintf("%v", results.Rows))
}

// TestScalarGroupConcat tests group_concat with partial aggregation on engine.
func TestScalarGroupConcat(t *testing.T) {
	fields := sqltypes.MakeTestFields(
		"group_concat(c2)",
		"text",
	)

	varbinaryFields := sqltypes.MakeTestFields(
		"group_concat(c2)",
		"blob",
	)

	tcases := []struct {
		name        string
		inputResult *sqltypes.Result
		expResult   *sqltypes.Result
	}{{
		name: "ending with null",
		inputResult: sqltypes.MakeTestResult(fields,
			"a", "a", "b", "null", "null"),
		expResult: sqltypes.MakeTestResult(fields,
			`a,a,b`),
	}, {
		name:        "empty result",
		inputResult: sqltypes.MakeTestResult(fields),
		expResult: sqltypes.MakeTestResult(fields,
			`null`),
	}, {
		name: "only null value for concat",
		inputResult: sqltypes.MakeTestResult(fields,
			"null", "null", "null"),
		expResult: sqltypes.MakeTestResult(fields,
			`null`),
	}, {
		name: "empty string value for concat",
		inputResult: sqltypes.MakeTestResult(fields,
			"", "", ""),
		expResult: sqltypes.MakeTestResult(fields,
			`,,`),
	}, {
		name: "varbinary column",
		inputResult: sqltypes.MakeTestResult(varbinaryFields,
			"foo", "null", "bar"),
		expResult: sqltypes.MakeTestResult(varbinaryFields,
			`foo,bar`),
	}}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			fp := &fakePrimitive{results: []*sqltypes.Result{tcase.inputResult}}
			oa := &ScalarAggregate{
				Aggregates: []*AggregateParams{{
					Opcode: AggregateGroupConcat,
					Col:    0,
					Func:   &sqlparser.GroupConcatExpr{Separator: ","},
				}},
				Input: fp,
			}
			qr, err := oa.TryExecute(context.Background(), &noopVCursor{}, nil, false)
			require.NoError(t, err)
			assert.Equal(t, tcase.expResult, qr)

			fp.rewind()
			results := &sqltypes.Result{}
			err = oa.TryStreamExecute(context.Background(), &noopVCursor{}, nil, true, func(qr *sqltypes.Result) error {
				if qr.Fields != nil {
					results.Fields = qr.Fields
				}
				results.Rows = append(results.Rows, qr.Rows...)
				return nil
			})
			require.NoError(t, err)
			assert.Equal(t, tcase.expResult, results)
		})
	}
}

// TestScalarAggregatePassThroughColumnNullable: without a GROUP BY, a column
// the aggregate passes through (not aggregated, or any_value) is NULL over an
// empty input, so it is nullable even when its input column is NOT NULL, as
// MySQL reports it. A count stays NOT NULL. Every path that builds fields
// agrees.
func TestScalarAggregatePassThroughColumnNullable(t *testing.T) {
	notNull := uint32(querypb.MySqlFlag_NOT_NULL_FLAG)
	input := func() *fakePrimitive {
		f := sqltypes.MakeTestFields("a|b|c", "int64|int64|int64")
		for _, fl := range f {
			fl.Flags |= notNull
		}
		return &fakePrimitive{results: []*sqltypes.Result{sqltypes.MakeTestResult(f)}}
	}
	newAggr := func() *ScalarAggregate {
		return &ScalarAggregate{
			Aggregates: []*AggregateParams{
				NewAggregateParam(AggregateAnyValue, 1, nil, "b", collations.MySQL8()),
				NewAggregateParam(AggregateCount, 2, nil, "c", collations.MySQL8()),
			},
			Input: input(),
		}
	}
	want := []uint32{0, 0, notNull}
	flags := func(fields []*querypb.Field) []uint32 {
		out := make([]uint32, len(fields))
		for i, f := range fields {
			out[i] = f.Flags & notNull
		}
		return out
	}

	r, err := newAggr().TryExecute(context.Background(), &noopVCursor{}, nil, true)
	require.NoError(t, err)
	require.True(t, r.Rows[0][0].IsNull())
	require.True(t, r.Rows[0][1].IsNull())
	require.Equal(t, want, flags(r.Fields), "TryExecute")

	r, err = newAggr().GetFields(context.Background(), &noopVCursor{}, nil)
	require.NoError(t, err)
	require.Equal(t, want, flags(r.Fields), "GetFields")

	var streamed []*querypb.Field
	err = newAggr().TryStreamExecute(context.Background(), &noopVCursor{}, nil, true, func(qr *sqltypes.Result) error {
		if streamed == nil && len(qr.Fields) != 0 {
			streamed = qr.Fields
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, want, flags(streamed), "TryStreamExecute")
}
