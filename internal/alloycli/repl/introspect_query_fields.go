package repl

import (
	"encoding/json"

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
	Name        string `json:"name"`
	Description string `json:"description"`
	Args        []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"args"`
}

func IntrospectQueryFields(gqlClient *graphql.GraphQlClient) ([]IntrospectionQueryField, error) {
	response, err := gqlClient.Execute(`
		query {
			__schema {
				queryType {
					fields {
						name
						description
						args {
							name
							description
						}
					}
				}
			}
		}
	`)
	if err != nil {
		return []IntrospectionQueryField{}, err
	}

	var introspectionQueryResponse IntrospectionQuery
	err = json.Unmarshal(response.Raw, &introspectionQueryResponse)
	if err != nil {
		return []IntrospectionQueryField{}, err
	}

	return introspectionQueryResponse.Data.Schema.QueryType.Fields, nil
}
