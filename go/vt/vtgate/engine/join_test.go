/*
Copyright 2019 The Vitess Authors.

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
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"vitess.io/vitess/go/sqltypes"

	querypb "vitess.io/vitess/go/vt/proto/query"
)

func TestJoinExecute(t *testing.T) {
	leftPrim := &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				sqltypes.MakeTestFields(
					"col1|col2|col3",
					"int64|varchar|varchar",
				),
				"1|a|aa",
				"2|b|bb",
				"3|c|cc",
			),
		},
	}
	rightFields := sqltypes.MakeTestFields(
		"col4|col5|col6",
		"int64|varchar|varchar",
	)
	rightPrim := &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				rightFields,
				"4|d|dd",
			),
			sqltypes.MakeTestResult(
				rightFields,
			),
			sqltypes.MakeTestResult(
				rightFields,
				"5|e|ee",
				"6|f|ff",
				"7|g|gg",
			),
		},
	}
	bv := map[string]*querypb.BindVariable{
		"a": sqltypes.Int64BindVariable(10),
	}

	// Normal join
	jn := &Join{
		Opcode: InnerJoin,
		Left:   leftPrim,
		Right:  rightPrim,
		Cols:   []int{-1, -2, 1, 2},
		Vars: map[string]int{
			"bv": 1,
		},
	}
	r, err := jn.TryExecute(context.Background(), &noopVCursor{}, bv, true)
	if err != nil {
		t.Fatal(err)
	}
	leftPrim.ExpectLog(t, []string{
		fmt.Sprintf(`Execute %v true`, printBindVars(map[string]*querypb.BindVariable{"a": sqltypes.Int64BindVariable(10)})),
	})
	rightPrim.ExpectLog(t, []string{
		fmt.Sprintf(`Execute %v true`, printBindVars(map[string]*querypb.BindVariable{"a": sqltypes.Int64BindVariable(10), "bv": sqltypes.StringBindVariable("a")})),
		fmt.Sprintf(`Execute %v false`, printBindVars(map[string]*querypb.BindVariable{"a": sqltypes.Int64BindVariable(10), "bv": sqltypes.StringBindVariable("b")})),
		fmt.Sprintf(`Execute %v false`, printBindVars(map[string]*querypb.BindVariable{"a": sqltypes.Int64BindVariable(10), "bv": sqltypes.StringBindVariable("c")})),
	})
	expectResult(t, r, sqltypes.MakeTestResult(
		sqltypes.MakeTestFields(
			"col1|col2|col4|col5",
			"int64|varchar|int64|varchar",
		),
		"1|a|4|d",
		"3|c|5|e",
		"3|c|6|f",
		"3|c|7|g",
	))

	// Left Join
	leftPrim.rewind()
	rightPrim.rewind()
	jn.Opcode = LeftJoin
	r, err = jn.TryExecute(context.Background(), &noopVCursor{}, bv, true)
	if err != nil {
		t.Fatal(err)
	}
	leftPrim.ExpectLog(t, []string{
		fmt.Sprintf(`Execute %v true`, printBindVars(map[string]*querypb.BindVariable{"a": sqltypes.Int64BindVariable(10)})),
	})
	rightPrim.ExpectLog(t, []string{
		fmt.Sprintf(`Execute %v true`, printBindVars(map[string]*querypb.BindVariable{"a": sqltypes.Int64BindVariable(10), "bv": sqltypes.StringBindVariable("a")})),
		fmt.Sprintf(`Execute %v false`, printBindVars(map[string]*querypb.BindVariable{"a": sqltypes.Int64BindVariable(10), "bv": sqltypes.StringBindVariable("b")})),
		fmt.Sprintf(`Execute %v false`, printBindVars(map[string]*querypb.BindVariable{"a": sqltypes.Int64BindVariable(10), "bv": sqltypes.StringBindVariable("c")})),
	})
	expectResult(t, r, sqltypes.MakeTestResult(
		sqltypes.MakeTestFields(
			"col1|col2|col4|col5",
			"int64|varchar|int64|varchar",
		),
		"1|a|4|d",
		"2|b|null|null",
		"3|c|5|e",
		"3|c|6|f",
		"3|c|7|g",
	))
}

func TestJoinExecuteMaxMemoryRows(t *testing.T) {
	saveMax := testMaxMemoryRows
	saveIgnore := testIgnoreMaxMemoryRows
	testMaxMemoryRows = 3
	defer func() {
		testMaxMemoryRows = saveMax
		testIgnoreMaxMemoryRows = saveIgnore
	}()

	testCases := []struct {
		ignoreMaxMemoryRows bool
		err                 string
	}{
		{true, ""},
		{false, "in-memory row count exceeded allowed limit of 3"},
	}
	for _, test := range testCases {
		leftPrim := &fakePrimitive{
			results: []*sqltypes.Result{
				sqltypes.MakeTestResult(
					sqltypes.MakeTestFields(
						"col1|col2|col3",
						"int64|varchar|varchar",
					),
					"1|a|aa",
					"2|b|bb",
					"3|c|cc",
				),
			},
		}
		rightFields := sqltypes.MakeTestFields(
			"col4|col5|col6",
			"int64|varchar|varchar",
		)
		rightPrim := &fakePrimitive{
			results: []*sqltypes.Result{
				sqltypes.MakeTestResult(
					rightFields,
					"4|d|dd",
				),
				sqltypes.MakeTestResult(
					rightFields,
				),
				sqltypes.MakeTestResult(
					rightFields,
					"5|e|ee",
					"6|f|ff",
					"7|g|gg",
				),
			},
		}
		bv := map[string]*querypb.BindVariable{
			"a": sqltypes.Int64BindVariable(10),
		}

		// Normal join
		jn := &Join{
			Opcode: InnerJoin,
			Left:   leftPrim,
			Right:  rightPrim,
			Cols:   []int{-1, -2, 1, 2},
			Vars: map[string]int{
				"bv": 1,
			},
		}
		testIgnoreMaxMemoryRows = test.ignoreMaxMemoryRows
		_, err := jn.TryExecute(context.Background(), &noopVCursor{}, bv, true)
		if testIgnoreMaxMemoryRows {
			require.NoError(t, err)
		} else {
			require.EqualError(t, err, test.err)
		}
	}
}

func TestJoinExecuteNoResult(t *testing.T) {
	leftPrim := &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				sqltypes.MakeTestFields(
					"col1|col2|col3",
					"int64|varchar|varchar",
				),
			),
		},
	}
	rightFields := sqltypes.MakeTestFields(
		"col4|col5|col6",
		"int64|varchar|varchar",
	)
	rightPrim := &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				rightFields,
			),
		},
	}

	jn := &Join{
		Opcode: InnerJoin,
		Left:   leftPrim,
		Right:  rightPrim,
		Cols:   []int{-1, -2, 1, 2},
		Vars: map[string]int{
			"bv": 1,
		},
	}
	r, err := jn.TryExecute(context.Background(), &noopVCursor{}, map[string]*querypb.BindVariable{}, true)
	require.NoError(t, err)
	leftPrim.ExpectLog(t, []string{
		`Execute  true`,
	})
	rightPrim.ExpectLog(t, []string{
		`GetFields bv: `,
		`Execute bv:  true`,
	})
	wantResult := sqltypes.MakeTestResult(
		sqltypes.MakeTestFields(
			"col1|col2|col4|col5",
			"int64|varchar|int64|varchar",
		),
	)
	expectResult(t, r, wantResult)
}

func TestJoinExecuteErrors(t *testing.T) {
	// Error on left query
	leftPrim := &fakePrimitive{
		sendErr: errors.New("left err"),
	}

	jn := &Join{
		Opcode: InnerJoin,
		Left:   leftPrim,
	}
	_, err := jn.TryExecute(context.Background(), &noopVCursor{}, map[string]*querypb.BindVariable{}, true)
	require.EqualError(t, err, "left err")

	// Error on right query
	leftPrim = &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				sqltypes.MakeTestFields(
					"col1|col2|col3",
					"int64|varchar|varchar",
				),
				"1|a|aa",
				"2|b|bb",
				"3|c|cc",
			),
		},
	}
	rightPrim := &fakePrimitive{
		sendErr: errors.New("right err"),
	}

	jn = &Join{
		Opcode: InnerJoin,
		Left:   leftPrim,
		Right:  rightPrim,
		Cols:   []int{-1, -2, 1, 2},
		Vars: map[string]int{
			"bv": 1,
		},
	}
	_, err = jn.TryExecute(context.Background(), &noopVCursor{}, map[string]*querypb.BindVariable{}, true)
	require.EqualError(t, err, "right err")

	// Error on right getfields
	leftPrim = &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				sqltypes.MakeTestFields(
					"col1|col2|col3",
					"int64|varchar|varchar",
				),
			),
		},
	}
	rightPrim = &fakePrimitive{
		sendErr: errors.New("right err"),
	}

	jn = &Join{
		Opcode: InnerJoin,
		Left:   leftPrim,
		Right:  rightPrim,
		Cols:   []int{-1, -2, 1, 2},
		Vars: map[string]int{
			"bv": 1,
		},
	}
	_, err = jn.TryExecute(context.Background(), &noopVCursor{}, map[string]*querypb.BindVariable{}, true)
	require.EqualError(t, err, "right err")
}

func TestJoinStreamExecute(t *testing.T) {
	leftPrim := &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				sqltypes.MakeTestFields(
					"col1|col2|col3",
					"int64|varchar|varchar",
				),
				"1|a|aa",
				"2|b|bb",
				"3|c|cc",
			),
		},
	}
	rightFields := sqltypes.MakeTestFields(
		"col4|col5|col6",
		"int64|varchar|varchar",
	)
	rightPrim := &fakePrimitive{
		results: []*sqltypes.Result{
			// First right query will always be a GetFields.
			sqltypes.MakeTestResult(
				rightFields,
			),
			sqltypes.MakeTestResult(
				rightFields,
				"4|d|dd",
			),
			sqltypes.MakeTestResult(
				rightFields,
			),
			sqltypes.MakeTestResult(
				rightFields,
				"5|e|ee",
				"6|f|ff",
				"7|g|gg",
			),
		},
	}

	// Normal join
	jn := &Join{
		Opcode: InnerJoin,
		Left:   leftPrim,
		Right:  rightPrim,
		Cols:   []int{-1, -2, 1, 2},
		Vars: map[string]int{
			"bv": 1,
		},
	}
	r, err := wrapStreamExecute(jn, &noopVCursor{}, map[string]*querypb.BindVariable{}, true)
	if err != nil {
		t.Fatal(err)
	}
	leftPrim.ExpectLog(t, []string{
		`StreamExecute  true`,
	})
	rightPrim.ExpectLog(t, []string{
		`GetFields bv: `,
		`Execute bv:  true`,
		fmt.Sprintf(`StreamExecute %v false`, printBindVars(map[string]*querypb.BindVariable{"bv": sqltypes.StringBindVariable("a")})),
		fmt.Sprintf(`StreamExecute %v false`, printBindVars(map[string]*querypb.BindVariable{"bv": sqltypes.StringBindVariable("b")})),
		fmt.Sprintf(`StreamExecute %v false`, printBindVars(map[string]*querypb.BindVariable{"bv": sqltypes.StringBindVariable("c")})),
	})
	expectResult(t, r, sqltypes.MakeTestResult(
		sqltypes.MakeTestFields(
			"col1|col2|col4|col5",
			"int64|varchar|int64|varchar",
		),
		"1|a|4|d",
		"3|c|5|e",
		"3|c|6|f",
		"3|c|7|g",
	))

	// Left Join
	leftPrim.rewind()
	rightPrim.rewind()
	jn.Opcode = LeftJoin
	r, err = wrapStreamExecute(jn, &noopVCursor{}, map[string]*querypb.BindVariable{}, true)
	if err != nil {
		t.Fatal(err)
	}
	leftPrim.ExpectLog(t, []string{
		`StreamExecute  true`,
	})
	rightPrim.ExpectLog(t, []string{
		`GetFields bv: `,
		`Execute bv:  true`,
		fmt.Sprintf(`StreamExecute %v false`, printBindVars(map[string]*querypb.BindVariable{"bv": sqltypes.StringBindVariable("a")})),
		fmt.Sprintf(`StreamExecute %v false`, printBindVars(map[string]*querypb.BindVariable{"bv": sqltypes.StringBindVariable("b")})),
		fmt.Sprintf(`StreamExecute %v false`, printBindVars(map[string]*querypb.BindVariable{"bv": sqltypes.StringBindVariable("c")})),
	})
	expectResult(t, r, sqltypes.MakeTestResult(
		sqltypes.MakeTestFields(
			"col1|col2|col4|col5",
			"int64|varchar|int64|varchar",
		),
		"1|a|4|d",
		"2|b|null|null",
		"3|c|5|e",
		"3|c|6|f",
		"3|c|7|g",
	))
}

func TestGetFields(t *testing.T) {
	leftPrim := &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				sqltypes.MakeTestFields(
					"col1|col2|col3",
					"int64|varchar|varchar",
				),
			),
		},
	}
	rightFields := sqltypes.MakeTestFields(
		"col4|col5|col6",
		"int64|varchar|varchar",
	)
	rightPrim := &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				rightFields,
			),
		},
	}

	jn := &Join{
		Opcode: InnerJoin,
		Left:   leftPrim,
		Right:  rightPrim,
		Cols:   []int{-1, -2, 1, 2},
		Vars: map[string]int{
			"bv": 1,
		},
	}
	r, err := jn.GetFields(context.Background(), nil, map[string]*querypb.BindVariable{})
	if err != nil {
		t.Fatal(err)
	}
	leftPrim.ExpectLog(t, []string{
		`GetFields `,
		`Execute  true`,
	})
	rightPrim.ExpectLog(t, []string{
		`GetFields bv: `,
		`Execute bv:  true`,
	})
	expectResult(t, r, sqltypes.MakeTestResult(
		sqltypes.MakeTestFields(
			"col1|col2|col4|col5",
			"int64|varchar|int64|varchar",
		),
	))
}

func TestGetFieldsErrors(t *testing.T) {
	leftPrim := &fakePrimitive{
		sendErr: errors.New("left err"),
	}
	rightPrim := &fakePrimitive{
		sendErr: errors.New("right err"),
	}

	jn := &Join{
		Opcode: InnerJoin,
		Left:   leftPrim,
		Right:  rightPrim,
		Cols:   []int{-1, -2, 1, 2},
		Vars: map[string]int{
			"bv": 1,
		},
	}
	_, err := jn.GetFields(context.Background(), nil, map[string]*querypb.BindVariable{})
	require.EqualError(t, err, "left err")

	jn.Left = &fakePrimitive{
		results: []*sqltypes.Result{
			sqltypes.MakeTestResult(
				sqltypes.MakeTestFields(
					"col1|col2|col3",
					"int64|varchar|varchar",
				),
			),
		},
	}
	_, err = jn.GetFields(context.Background(), nil, map[string]*querypb.BindVariable{})
	require.EqualError(t, err, "right err")
}

// TestJoinFieldsLeftJoinNullable: a left join's right-side columns are
// nullable even when their source columns are NOT NULL, since unmatched rows
// are padded with NULL. The inputs' fields are left as they were.
func TestJoinFieldsLeftJoinNullable(t *testing.T) {
	notNull := uint32(querypb.MySqlFlag_NOT_NULL_FLAG)
	unsigned := uint32(querypb.MySqlFlag_UNSIGNED_FLAG)
	lfields := []*querypb.Field{{Name: "l", Type: sqltypes.Int64, Flags: notNull}}
	rfields := []*querypb.Field{{Name: "r", Type: sqltypes.Uint64, Flags: notNull | unsigned}}
	cols := []int{-1, 1}

	inner := joinFields(lfields, rfields, cols, InnerJoin)
	require.Equal(t, notNull, inner[0].Flags)
	require.Equal(t, notNull|unsigned, inner[1].Flags)

	left := joinFields(lfields, rfields, cols, LeftJoin)
	require.Equal(t, notNull, left[0].Flags, "left-side column keeps NOT NULL")
	require.Equal(t, unsigned, left[1].Flags, "right-side column loses only NOT NULL")
	require.Equal(t, notNull|unsigned, rfields[0].Flags, "input field not modified")
}

// TestBindvarForTypeMediumInt: an empty left input binds a MEDIUMINT join
// variable as a numeric placeholder, like every other integer type, not NULL.
func TestBindvarForTypeMediumInt(t *testing.T) {
	for _, typ := range []querypb.Type{sqltypes.Int24, sqltypes.Uint24} {
		bv := bindvarForType(&querypb.Field{Type: typ})
		require.Equal(t, typ, bv.Type, typ.String())
		require.Equal(t, []byte("0"), bv.Value, typ.String())
	}
}
