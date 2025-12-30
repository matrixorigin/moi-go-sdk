package sdk

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivLiveFlow(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	catalogID, markCatalogDeleted := createTestCatalog(t, client)

	// Create a role with permissions
	roleID, markRoleDeleted := createTestRole(t, client, []string{string(PrivCode_QueryCatalog), string(PrivCode_UpdateCatalog)})

	// Create a user with the role
	createResp, err := client.CreateUser(ctx, &UserCreateRequest{
		UserName:    strings.ToLower(randomUserName()),
		Password:    "TestPwd123!",
		RoleIDList:  []RoleID{roleID},
		Description: "sdk test user for priv check",
		Phone:       "12345678901",
		Email:       "sdk-priv@example.com",
	})
	require.NoError(t, err)
	userID := createResp.UserID

	userDeleted := false
	t.Cleanup(func() {
		if userDeleted {
			return
		}
		if _, err := client.DeleteUser(ctx, &UserDeleteUserRequest{UserID: userID}); err != nil {
			t.Logf("cleanup delete user failed: %v", err)
		}
		if _, err := client.DeleteRole(ctx, &RoleDeleteRequest{RoleID: roleID}); err != nil {
			t.Logf("cleanup delete role failed: %v", err)
		}
		markRoleDeleted()
	})

	// List objects by category with uid header
	// listReq := &PrivListObjByCategoryRequest{ObjType: ObjTypeCatalog.String()}
	// listResp, err := client.ListObjectsByCategory(ctx, listReq)
	// require.NoError(t, err)
	// require.NotNil(t, listResp)

	// Cleanup
	_, err = client.DeleteUser(ctx, &UserDeleteUserRequest{UserID: userID})
	require.NoError(t, err)
	userDeleted = true

	_, err = client.DeleteRole(ctx, &RoleDeleteRequest{RoleID: roleID})
	require.NoError(t, err)
	markRoleDeleted()

	_, err = client.DeleteCatalog(ctx, &CatalogDeleteRequest{CatalogID: catalogID})
	require.NoError(t, err)
	markCatalogDeleted()
}

func TestPrivNilRequestErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := &RawClient{}

	tests := []struct {
		name string
		call func() error
	}{
		{"ListByCategory", func() error { _, err := client.ListObjectsByCategory(ctx, nil); return err }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.call(), ErrNilRequest)
		})
	}
}

// TestTableRowColExpression_JSON 测试 TableRowColExpression 的 JSON 序列化和反序列化
func TestTableRowColExpression_JSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expr      *TableRowColExpression
		jsonStr   string
		wantEqual bool
	}{
		{
			name: "single expression value",
			expr: &TableRowColExpression{
				Operator:   "=",
				Expression: []string{"100"},
				MatchType:  "i",
			},
			jsonStr:   `{"operator":"=","expression":["100"],"match_type":"i"}`,
			wantEqual: true,
		},
		{
			name: "multiple expression values",
			expr: &TableRowColExpression{
				Operator:   "in",
				Expression: []string{"IT", "HR", "Finance"},
				MatchType:  "c",
			},
			jsonStr:   `{"operator":"in","expression":["IT","HR","Finance"],"match_type":"c"}`,
			wantEqual: true,
		},
		{
			name: "regexp_like operator",
			expr: &TableRowColExpression{
				Operator:   "regexp_like",
				Expression: []string{"^test.*"},
				MatchType:  "i",
			},
			jsonStr:   `{"operator":"regexp_like","expression":["^test.*"],"match_type":"i"}`,
			wantEqual: true,
		},
		{
			name: "like operator with multiple patterns",
			expr: &TableRowColExpression{
				Operator:   "like",
				Expression: []string{"%test%", "%demo%"},
				MatchType:  "i",
			},
			jsonStr:   `{"operator":"like","expression":["%test%","%demo%"],"match_type":"i"}`,
			wantEqual: true,
		},
		{
			name: "comparison operators",
			expr: &TableRowColExpression{
				Operator:   ">=",
				Expression: []string{"100"},
				MatchType:  "n",
			},
			jsonStr:   `{"operator":">=","expression":["100"],"match_type":"n"}`,
			wantEqual: true,
		},
		{
			name: "not equal operator",
			expr: &TableRowColExpression{
				Operator:   "!=",
				Expression: []string{"deleted"},
				MatchType:  "c",
			},
			jsonStr:   `{"operator":"!=","expression":["deleted"],"match_type":"c"}`,
			wantEqual: true,
		},
		{
			name: "empty expression array",
			expr: &TableRowColExpression{
				Operator:   "=",
				Expression: []string{},
				MatchType:  "i",
			},
			jsonStr:   `{"operator":"=","expression":[],"match_type":"i"}`,
			wantEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.expr)
			require.NoError(t, err)
			require.JSONEq(t, tt.jsonStr, string(jsonData))

			// Test unmarshaling
			var unmarshaled TableRowColExpression
			err = json.Unmarshal([]byte(tt.jsonStr), &unmarshaled)
			require.NoError(t, err)
			require.Equal(t, tt.expr.Operator, unmarshaled.Operator)
			require.Equal(t, tt.expr.Expression, unmarshaled.Expression)
			require.Equal(t, tt.expr.MatchType, unmarshaled.MatchType)
		})
	}
}

// TestAuthorityCodeAndRule_JSON 测试 AuthorityCodeAndRule 的 JSON 序列化和反序列化
func TestAuthorityCodeAndRule_JSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		auth      *AuthorityCodeAndRule
		jsonStr   string
		wantEqual bool
	}{
		{
			name: "with rule list containing expression array",
			auth: &AuthorityCodeAndRule{
				Code:            "DT8",
				BlackColumnList: []string{"salary", "ssn"},
				RuleList: []*TableRowColRule{
					{
						Column:   "department",
						Relation: "and",
						ExpressionList: []*TableRowColExpression{
							{
								Operator:   "=",
								Expression: []string{"IT"},
								MatchType:  "i",
							},
						},
					},
				},
			},
			jsonStr:   `{"code":"DT8","black_column_list":["salary","ssn"],"rule_list":[{"column":"department","relation":"and","expression_list":[{"operator":"=","expression":["IT"],"match_type":"i"}]}]}`,
			wantEqual: true,
		},
		{
			name: "with multiple expressions in rule",
			auth: &AuthorityCodeAndRule{
				Code:            "DT9",
				BlackColumnList: nil,
				RuleList: []*TableRowColRule{
					{
						Column:   "id",
						Relation: "and",
						ExpressionList: []*TableRowColExpression{
							{
								Operator:   "in",
								Expression: []string{"1", "2", "3"},
								MatchType:  "n",
							},
							{
								Operator:   ">",
								Expression: []string{"0"},
								MatchType:  "n",
							},
						},
					},
				},
			},
			jsonStr:   `{"code":"DT9","black_column_list":null,"rule_list":[{"column":"id","relation":"and","expression_list":[{"operator":"in","expression":["1","2","3"],"match_type":"n"},{"operator":">","expression":["0"],"match_type":"n"}]}]}`,
			wantEqual: true,
		},
		{
			name: "without rule list",
			auth: &AuthorityCodeAndRule{
				Code:            "DT10",
				BlackColumnList: []string{},
				RuleList:        nil,
			},
			jsonStr:   `{"code":"DT10","black_column_list":[],"rule_list":null}`,
			wantEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.auth)
			require.NoError(t, err)
			require.JSONEq(t, tt.jsonStr, string(jsonData))

			// Test unmarshaling
			var unmarshaled AuthorityCodeAndRule
			err = json.Unmarshal([]byte(tt.jsonStr), &unmarshaled)
			require.NoError(t, err)
			require.Equal(t, tt.auth.Code, unmarshaled.Code)
			require.Equal(t, tt.auth.BlackColumnList, unmarshaled.BlackColumnList)

			if tt.auth.RuleList == nil {
				require.Nil(t, unmarshaled.RuleList)
			} else {
				require.Equal(t, len(tt.auth.RuleList), len(unmarshaled.RuleList))
				for i, rule := range tt.auth.RuleList {
					require.Equal(t, rule.Column, unmarshaled.RuleList[i].Column)
					require.Equal(t, rule.Relation, unmarshaled.RuleList[i].Relation)
					require.Equal(t, len(rule.ExpressionList), len(unmarshaled.RuleList[i].ExpressionList))
					for j, expr := range rule.ExpressionList {
						require.Equal(t, expr.Operator, unmarshaled.RuleList[i].ExpressionList[j].Operator)
						require.Equal(t, expr.Expression, unmarshaled.RuleList[i].ExpressionList[j].Expression)
						require.Equal(t, expr.MatchType, unmarshaled.RuleList[i].ExpressionList[j].MatchType)
					}
				}
			}
		})
	}
}

// TestObjPrivResponse_JSON 测试 ObjPrivResponse 的 JSON 序列化和反序列化
func TestObjPrivResponse_JSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		objPriv   *ObjPrivResponse
		jsonStr   string
		wantEqual bool
	}{
		{
			name: "complete object privilege response",
			objPriv: &ObjPrivResponse{
				ObjID:   "123",
				ObjType: "table",
				ObjName: "employees",
				AuthorityCodeList: []*AuthorityCodeAndRule{
					{
						Code:            "DT8",
						BlackColumnList: []string{"salary"},
						RuleList: []*TableRowColRule{
							{
								Column:   "department",
								Relation: "and",
								ExpressionList: []*TableRowColExpression{
									{
										Operator:   "=",
										Expression: []string{"IT"},
										MatchType:  "i",
									},
								},
							},
						},
					},
				},
			},
			jsonStr:   `{"id":"123","category":"table","name":"employees","authority_code_list":[{"code":"DT8","black_column_list":["salary"],"rule_list":[{"column":"department","relation":"and","expression_list":[{"operator":"=","expression":["IT"],"match_type":"i"}]}]}]}`,
			wantEqual: true,
		},
		{
			name: "with multiple authority codes and expressions",
			objPriv: &ObjPrivResponse{
				ObjID:   "456",
				ObjType: "table",
				ObjName: "orders",
				AuthorityCodeList: []*AuthorityCodeAndRule{
					{
						Code:            "DT8",
						BlackColumnList: nil,
						RuleList: []*TableRowColRule{
							{
								Column:   "status",
								Relation: "and",
								ExpressionList: []*TableRowColExpression{
									{
										Operator:   "in",
										Expression: []string{"pending", "processing"},
										MatchType:  "c",
									},
								},
							},
						},
					},
					{
						Code:            "DT9",
						BlackColumnList: []string{"price"},
						RuleList: []*TableRowColRule{
							{
								Column:   "user_id",
								Relation: "and",
								ExpressionList: []*TableRowColExpression{
									{
										Operator:   "regexp_like",
										Expression: []string{"^user_\\d+$"},
										MatchType:  "i",
									},
								},
							},
						},
					},
				},
			},
			jsonStr:   `{"id":"456","category":"table","name":"orders","authority_code_list":[{"code":"DT8","black_column_list":null,"rule_list":[{"column":"status","relation":"and","expression_list":[{"operator":"in","expression":["pending","processing"],"match_type":"c"}]}]},{"code":"DT9","black_column_list":["price"],"rule_list":[{"column":"user_id","relation":"and","expression_list":[{"operator":"regexp_like","expression":["^user_\\d+$"],"match_type":"i"}]}]}]}`,
			wantEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.objPriv)
			require.NoError(t, err)
			require.JSONEq(t, tt.jsonStr, string(jsonData))

			// Test unmarshaling
			var unmarshaled ObjPrivResponse
			err = json.Unmarshal([]byte(tt.jsonStr), &unmarshaled)
			require.NoError(t, err)
			require.Equal(t, tt.objPriv.ObjID, unmarshaled.ObjID)
			require.Equal(t, tt.objPriv.ObjType, unmarshaled.ObjType)
			require.Equal(t, tt.objPriv.ObjName, unmarshaled.ObjName)
			require.Equal(t, len(tt.objPriv.AuthorityCodeList), len(unmarshaled.AuthorityCodeList))

			for i, authCode := range tt.objPriv.AuthorityCodeList {
				require.Equal(t, authCode.Code, unmarshaled.AuthorityCodeList[i].Code)
				require.Equal(t, authCode.BlackColumnList, unmarshaled.AuthorityCodeList[i].BlackColumnList)
			}
		})
	}
}

// TestTableRowColExpression_ExpressionArray 专门测试 Expression 字段作为数组的各种情况
func TestTableRowColExpression_ExpressionArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		expression   []string
		expectedJSON string
		description  string
	}{
		{
			name:         "single value",
			expression:   []string{"100"},
			expectedJSON: `["100"]`,
			description:  "单个表达式值",
		},
		{
			name:         "multiple values",
			expression:   []string{"IT", "HR", "Finance"},
			expectedJSON: `["IT","HR","Finance"]`,
			description:  "多个表达式值",
		},
		{
			name:         "empty array",
			expression:   []string{},
			expectedJSON: `[]`,
			description:  "空数组",
		},
		{
			name:         "numeric values",
			expression:   []string{"1", "2", "3", "4", "5"},
			expectedJSON: `["1","2","3","4","5"]`,
			description:  "多个数字值",
		},
		{
			name:         "regex patterns",
			expression:   []string{"^test.*", ".*demo$", "pattern\\d+"},
			expectedJSON: `["^test.*",".*demo$","pattern\\d+"]`,
			description:  "正则表达式模式",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := &TableRowColExpression{
				Operator:   "=",
				Expression: tt.expression,
				MatchType:  "i",
			}

			// Test JSON marshaling
			jsonData, err := json.Marshal(expr)
			require.NoError(t, err, "should marshal without error")

			var unmarshaled struct {
				Operator   string   `json:"operator"`
				Expression []string `json:"expression"`
				MatchType  string   `json:"match_type"`
			}
			err = json.Unmarshal(jsonData, &unmarshaled)
			require.NoError(t, err, "should unmarshal without error")
			require.Equal(t, tt.expression, unmarshaled.Expression, "expression array should match")

			// Test that expression is indeed an array in JSON
			var jsonMap map[string]interface{}
			err = json.Unmarshal(jsonData, &jsonMap)
			require.NoError(t, err)
			exprValue, ok := jsonMap["expression"]
			require.True(t, ok, "expression field should exist")
			_, isArray := exprValue.([]interface{})
			require.True(t, isArray, "expression should be an array in JSON")
		})
	}
}
