package nixsearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
)

type Input struct {
	Channel string
	Query   string
}

type Output struct {
	Input    *Input
	Packages []Package
}

type Package struct {
	Name          string   `json:"package_pname"`
	AttrName      string   `json:"package_attr_name"`
	AttrSet       string   `json:"package_attr_set"`
	Outputs       []string `json:"package_outputs"`
	DefaultOutput *string  `json:"package_default_output"`
	Description   *string  `json:"package_description"`
	Programs      []string `json:"package_programs"`
	Homepage      []string `json:"package_homepage"`
	Version       string   `json:"package_pversion"`
	Platforms     []string `json:"package_platforms"`
	Position      string   `json:"package_position"`
	Licenses      []struct {
		FullName string  `json:"fullName"`
		URL      *string `json:"url"`
	}
}

type Hit struct {
	ID      string  `json:"_id"`
	Package Package `json:"_source"`
}

type Error struct {
	Type         string `json:"type"`
	Reason       string `json:"reason"`
	ResourceType string `json:"resource.type"`
	ResourceID   string `json:"resource.id"`
}
type Response struct {
	// set on error case
	Error  *Error `json:"error"`
	Status *int   `json:"status"`
	// set on success
	Hits struct {
		Hits []Hit `json:"hits"`
	} `json:"hits"`
}

func Search(input Input) (*Output, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.Logger = nil

	url := formatURL(input.Channel)
	payload, err := formatQuery(input.Query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(esUsername, esPassword)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := retryClient.StandardClient().Do(req)
	if err != nil {
		return nil, err
	}
	x, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var y Response
	if err := json.Unmarshal(x, &y); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		if y.Error == nil {
			return nil, fmt.Errorf("API failed with status=%d: %s", resp.StatusCode, x)
		}
		if y.Error.Type == "index_not_found_exception" {
			return nil, fmt.Errorf("API failed with status=%d: index=%s does not exist (invalid --channel=%s)", resp.StatusCode, y.Error.ResourceID, input.Channel)
		}
		return nil, fmt.Errorf(y.Error.Reason)
	}

	output := &Output{
		Input:    &input,
		Packages: make([]Package, len(y.Hits.Hits)),
	}
	for i, hit := range y.Hits.Hits {
		output.Packages[i] = hit.Package
	}

	return output, nil
}

func formatURL(channel string) string {
	return fmt.Sprintf(templateURL, url.QueryEscape(channel))
}

func formatQuery(query string) (string, error) {
	matchName := "multi_match_" + strings.ReplaceAll(query, " ", "_")
	encQuery, err := json.Marshal(query)
	if err != nil {
		return "", fmt.Errorf("failed to encode query: %w", err)
	}
	encMatchName, err := json.Marshal(matchName)
	if err != nil {
		return "", fmt.Errorf("failed to encode match name: %w", err)
	}
	value := fmt.Sprintf("*%s*", query)
	encValue, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to encode value: %w", err)
	}
	return fmt.Sprintf(templatePayload, encQuery, encMatchName, encValue), nil
}

const (
	// https://github.com/NixOS/nixos-search/blob/main/frontend/src/index.js
	esUsername      = "aWVSALXpZv"
	esPassword      = "X8gPHnzL52wFEekuxsfQ9cSh"
	templateURL     = `https://nixos-search-7-1733963800.us-east-1.bonsaisearch.net:443/latest-37-nixos-%s/_search`
	templatePayload = `
{
	"from": 0,
	"size": 50,
	"sort": [
	  {
		"_score": "desc",
		"package_attr_name": "desc",
		"package_pversion": "desc"
	  }
	],
	"aggs": {
	  "package_attr_set": {
		"terms": {
		  "field": "package_attr_set",
		  "size": 20
		}
	  },
	  "package_license_set": {
		"terms": {
		  "field": "package_license_set",
		  "size": 20
		}
	  },
	  "package_maintainers_set": {
		"terms": {
		  "field": "package_maintainers_set",
		  "size": 20
		}
	  },
	  "package_platforms": {
		"terms": {
		  "field": "package_platforms",
		  "size": 20
		}
	  },
	  "all": {
		"global": {},
		"aggregations": {
		  "package_attr_set": {
			"terms": {
			  "field": "package_attr_set",
			  "size": 20
			}
		  },
		  "package_license_set": {
			"terms": {
			  "field": "package_license_set",
			  "size": 20
			}
		  },
		  "package_maintainers_set": {
			"terms": {
			  "field": "package_maintainers_set",
			  "size": 20
			}
		  },
		  "package_platforms": {
			"terms": {
			  "field": "package_platforms",
			  "size": 20
			}
		  }
		}
	  }
	},
	"query": {
	  "bool": {
		"filter": [
		  {
			"term": {
			  "type": {
				"value": "package",
				"_name": "filter_packages"
			  }
			}
		  },
		  {
			"bool": {
			  "must": [
				{
				  "bool": {
					"should": []
				  }
				},
				{
				  "bool": {
					"should": []
				  }
				},
				{
				  "bool": {
					"should": []
				  }
				},
				{
				  "bool": {
					"should": []
				  }
				}
			  ]
			}
		  }
		],
		"must": [
		  {
			"dis_max": {
			  "tie_breaker": 0.7,
			  "queries": [
				{
				  "multi_match": {
					"type": "cross_fields",
					"query": %s,
					"analyzer": "whitespace",
					"auto_generate_synonyms_phrase_query": false,
					"operator": "and",
					"_name": %s,
					"fields": [
					  "package_attr_name^9",
					  "package_attr_name.*^5.3999999999999995",
					  "package_programs^9",
					  "package_programs.*^5.3999999999999995",
					  "package_pname^6",
					  "package_pname.*^3.5999999999999996",
					  "package_description^1.3",
					  "package_description.*^0.78",
					  "package_longDescription^1",
					  "package_longDescription.*^0.6",
					  "flake_name^0.5",
					  "flake_name.*^0.3"
					]
				  }
				},
				{
				  "wildcard": {
					"package_attr_name": {
					  "value": %s,
					  "case_insensitive": true
					}
				  }
				}
			  ]
			}
		  }
		]
	  }
	}
  }
`
)
