package plugin

import "encoding/json"

// injectConnectionArgs sets the connectionArgs field on a query JSON body to
// the marshalled args (overwriting any existing value), preserving every other
// field. The returned body is a fresh allocation; the input is not modified.
//
// A nil body is treated as the empty object {} so callers don't have to
// pre-seed one. Malformed body bytes propagate the json.Unmarshal error
// verbatim.
func injectConnectionArgs(body json.RawMessage, args map[string]string) (json.RawMessage, error) {
	obj := map[string]json.RawMessage{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &obj); err != nil {
			return nil, err
		}
	}
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	obj["connectionArgs"] = argsJSON
	return json.Marshal(obj)
}
