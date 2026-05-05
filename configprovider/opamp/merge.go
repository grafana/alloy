package opamp

import (
	yamlv3 "gopkg.in/yaml.v3"
)

// mergeRoot merges src into dest (dest is updated).
func mergeRoot(dest, src map[string]any) {
	if src == nil {
		return
	}
	for k, sv := range src {
		if k == "service" {
			mergeServiceTop(dest, sv)
			continue
		}
		mergeKey(dest, k, sv)
	}
}

func mergeKey(dest map[string]any, k string, sv any) {
	dv, exists := dest[k]
	if !exists {
		dest[k] = deepCloneValue(sv)
		return
	}
	dm, okD := dv.(map[string]any)
	sm, okS := sv.(map[string]any)
	if okD && okS {
		mergeRoot(dm, sm)
		return
	}
	dest[k] = deepCloneValue(sv)
}

func mergeServiceTop(dest map[string]any, srcSvc any) {
	sm, ok := srcSvc.(map[string]any)
	if !ok {
		return
	}
	dSvc, ok := dest["service"].(map[string]any)
	if !ok || dSvc == nil {
		dest["service"] = deepCloneValue(sm).(map[string]any)
		return
	}
	for k, sv := range sm {
		switch k {
		case "extensions":
			mergeExtensionsList(dSvc, sv)
		case "pipelines":
			mergePipelines(dSvc, sv)
		default:
			mergeKey(dSvc, k, sv)
		}
	}
}

func mergeExtensionsList(dSvc map[string]any, srcExt any) {
	srcList := toAnySlice(srcExt)
	if len(srcList) == 0 {
		return
	}
	destList := toAnySlice(dSvc["extensions"])
	seen := make(map[any]struct{}, len(destList)+len(srcList))
	var out []any
	for _, x := range destList {
		if _, dup := seen[x]; !dup {
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	for _, x := range srcList {
		if _, dup := seen[x]; !dup {
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	dSvc["extensions"] = out
}

func mergePipelines(dSvc map[string]any, srcP any) {
	sm, ok := srcP.(map[string]any)
	if !ok {
		return
	}
	dp, _ := dSvc["pipelines"].(map[string]any)
	if dp == nil {
		dp = make(map[string]any)
		dSvc["pipelines"] = dp
	}
	for k, v := range sm {
		dp[k] = deepCloneValue(v)
	}
}

func toAnySlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out
	default:
		return nil
	}
}

func deepCloneValue(v any) any {
	b, err := yamlv3.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := yamlv3.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}
