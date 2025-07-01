package repl

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/alloy/internal/service/graphql"
)

type IntrospectionQuery struct {
	Data struct {
		Schema struct {
			QueryType struct {
				Fields []IntrospectionQueryField `json:"fields"`
			} `json:"queryType"`
		} `json:"__schema"`
	} `json:"data"`
}

type IntrospectionQueryField struct {
	Name string `json:"name"`
	Args []struct {
		Name string `json:"name"`
	} `json:"args"`
}

func IntrospectQueryFields(gqlClient *graphql.GraphQlClient) ([]IntrospectionQueryField, error) {
	response, err := gqlClient.Execute(`
		query {
			__schema {
				queryType {
					fields {
						name
						args {
							name
						}
					}
				}
			}
		}
	`)
	if err != nil {
		fmt.Printf("Error introspecting schema: %v\n", err)
		return []IntrospectionQueryField{}, err
	}

	var introspectionQueryResponse IntrospectionQuery
	err = json.Unmarshal(response.Raw, &introspectionQueryResponse)
	if err != nil {
		fmt.Printf("Error unmarshaling response: %v\n", err)
		return []IntrospectionQueryField{}, err
	}

	return introspectionQueryResponse.Data.Schema.QueryType.Fields, nil
}
