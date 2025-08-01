package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mitchellh/go-wordwrap"
)

// TODO: use x-destiny-component-type-dependency
// TODO: use x-documentation-attributes
// TODO: automatic ThrottleSecondsBetweenActionPerUser ?

var (
	specFile = flag.String("spec", "../api-src/openapi.json", "path to openapi spec (v3)")
)

var spec Spec

type Spec openapi3.T

var paths buf
var types buf
var helpers buf

var wantSchema = map[string]bool{}
var doneSchema = map[string]bool{}
var refToTypeOverride = map[string]string{
	"BaseItemComponentSetOfuint32": "BaseItemComponentSet[uint32]",

	"DestinyBaseItemComponentSetOfint32":  "BaseItemComponentSet[int32]",
	"DestinyBaseItemComponentSetOfint64":  "BaseItemComponentSet[int64]",
	"DestinyBaseItemComponentSetOfuint32": "BaseItemComponentSet[uint32]",

	"DestinyItemComponentSetOfint64":  "ItemComponentSet[int64]",
	"DestinyItemComponentSetOfint32":  "ItemComponentSet[int32]",
	"DestinyItemComponentSetOfuint32": "ItemComponentSet[uint32]",

	"ItemComponentSetOfint32":  "ItemComponentSet[int32]",
	"ItemComponentSetOfint64":  "ItemComponentSet[int64]",
	"ItemComponentSetOfuint32": "ItemComponentSet[uint32]",

	"DestinyVendorSaleItemSetComponentOfDestinyPublicVendorSaleItemComponent": "ItemComponentSet[PublicVendorSaleItemComponent]",
	"DestinyVendorSaleItemSetComponentOfDestinyVendorSaleItemComponent":       "ItemComponentSet[VendorSaleItemComponent]",

	"VendorItemComponentSetOfint32":        "ItemComponentSet[int32]",
	"DestinyVendorItemComponentSetOfint32": "ItemComponentSet[int32]",
}

func main() {
	flag.Parse()

	specBytes, err := os.ReadFile(*specFile)
	if err != nil {
		log.Fatal(err)
	}

	if err := json.Unmarshal(specBytes, &spec); err != nil {
		log.Fatal(err)
	}

	// Validate assumptions:
	// Check duplicate schemas
	checkDuplicateSchema(spec.Components.Schemas)

	// Do generic types
	handleGenerics(spec.Components.Schemas)

	responseType := func(ref string) string {
		ref = strings.TrimPrefix(ref, "#/components/responses/")
		l, err := spec.Components.Responses.JSONLookup(ref)
		if err != nil {
			panic(fmt.Errorf("couldnt find ref %s: %v", ref, err))
		}
		n := typeFromSchema(l.(*openapi3.Response).Content.Get("application/json").Schema.Value.Properties["Response"])
		return n
	}

	for _, url := range spec.Paths.InMatchingOrder() {
		path := spec.Paths.Find(url)
		operation := path.Get
		if operation == nil {
			operation = path.Post
		}
		if operation == nil {
			panic("unhandled operation")
		}
		method := methodName(path)
		var paramBuf, queryBuf buf
		methodParameters(&paths, method, operation, &paramBuf, &queryBuf)
		paths.Comment(`%s: %s`, method, path.Description)
		paths.Comment("")
		paths.Comment("URL: %s", url)
		paths.Comment("")
		paths.Comment("Operation: " + operation.OperationID)
		if operation.Deprecated {
			paths.Comment("")
			paths.Comment("Deprecated: see above.")
		}
		if operation.Security != nil {
			for _, req := range *operation.Security {
				for scheme, scopes := range req {
					paths.Comment("")
					paths.Comment(`Scope: %s %v`, scheme, scopes)
				}
			}
		}
		responseIdent := responseType(operation.Responses.Status(200).Ref)
		paths.Out(`func (a API) %s(ctx context.Context, req %sRequest) (*ServerResponse[%s], error) {`, method, method, responseIdent)
		paths.Debug(operation)
		paths.Out(`var resp ServerResponse[%s]`, responseIdent)
		paths.Out(`err := a.client.Do(ctx, ClientRequest{Operation:%q,`, operation.OperationID)
		if path.Get != nil {
			paths.Out("Method:%q,", "GET")
		} else if path.Post != nil {
			paths.Out("Method:%q,", "POST")
		}
		paths.Out(`PathSpec:%q, PathParams:map[string]string{`, url)
		paths.Out(paramBuf.String())
		paths.Out("}, QueryParams: url.Values{")
		paths.Out(queryBuf.String())
		if operation.RequestBody == nil {
			paths.Out("}}, &resp)")
		} else {
			paths.Out("}, Body:req.Body}, &resp)")
		}
		paths.Out("return &resp, err")
		paths.Out(`}`)
	}

	// TODO: output hash types

	for {
		updated := 0

		for _, ref := range orderedKeys(spec.Components.Schemas) {
			schema := spec.Components.Schemas[ref]
			if refToTypeOverride[ref] != "" {
				continue
			}
			ident := refToIdent(ref)
			if !wantSchema[ident] {
				continue
			}
			if doneSchema[ident] {
				continue
			}

			updated++
			doneSchema[ident] = true

			types.Comment(ref)
			types.Comment("")
			types.Comment(schema.Value.Description)

			if schema.Value.Type.Is("object") {
				if len(schema.Value.Properties) == 0 {
					types.Out("type %s map[string]any", ident)
					types.Debug(schema)
					continue
				}
				if ident == "ComponentResponse" {
					types.Out(`type %s[T any] struct{`, ident)
					types.Out("Data T `json:\"data\"`")
				} else if ident == "SearchResult" {
					types.Out(`type %s[T any] struct{`, ident)
					types.Out("Results []T `json:\"results\"`")
				} else {
					types.Out(`type %s struct {`, ident)
				}
				// def, _ := json.MarshalIndent(schema, "", " ")
				// types.Comment(string(def))
				types.Debug(schema)

				// if table, ok := schema.Value.Extensions["x-mobile-manifest-name"]; ok {
				// table := table.(string)
				if strings.HasPrefix(ref, "Destiny.Definitions.") {
					parts := strings.Split(ref, ".")
					table := parts[len(parts)-1]
					helpers.Out("func (d %s) DefinitionTable() string {", ident)
					helpers.Out("return %q", table)
					helpers.Out("}")
				}
				// }

				for _, fieldName := range orderedKeys(schema.Value.Properties) {
					prop := schema.Value.Properties[fieldName]
					types.Out("")
					if prop.Value != nil && prop.Value.Description != "" {
						types.Comment(prop.Value.Description)
					}
					fieldType := typeFromSchema(prop)
					structOpt := ""
					// if fieldType == "int64" || fieldType == "Nullable[int64]" {
					// 	structOpt = ",string"
					// }
					if prop.Value != nil && prop.Value.Nullable {
						structOpt += ",omitempty"
					}
					structTag := fmt.Sprintf(`json:"%s%s"`, fieldName, structOpt)
					// if ident == "GroupResponse" && fieldName == "currentUserMemberMap" {
					// 	// https://github.com/Bungie-net/api/issues/1374
					// 	fieldType = "map[string]GroupMember"
					// }
					// if ident == "PlatformSilverComponent" && fieldName == "platformSilver" {
					// 	// https://github.com/Bungie-net/api/issues/1374
					// 	fieldType = "map[string]ItemComponent"
					// }
					types.Out("%s %s `%s`", capitalize(fieldName), fieldType, structTag)

					// if prop.Value != nil {
					// 	if ext, ok := prop.Value.Extensions["x-mapped-definition"]; ok {
					// 		mappedTo := ext.(map[string]any)["$ref"].(string)
					// 		types.Comment("Mapped to %s", mappedTo)
					// 	}
					// }
				}
				types.Out(`}`)
			} else if schema.Value.Enum != nil {
				typeAlias := typeFromSchema(schema)
				// def, _ := json.MarshalIndent(schema, "", " ")
				// types.Comment(string(def))
				types.Out(`type %s %s`, ident, typeAlias)

				if values, ok := schema.Value.Extensions["x-enum-values"]; ok {
					helpers.Out("")
					helpers.Out("func (e %s) Enum() string {", ident)
					helpers.Out("switch e {")
					types.Out("const (")
					values := values.([]interface{})
					for _, val := range values {
						val := val.(map[string]interface{})
						valueIdent := val["identifier"].(string)
						valueNumeric := val["numericValue"].(string)
						types.Out("%s_%s = %s(%s)", ident, valueIdent, ident, valueNumeric)
						helpers.Out("case %s_%s:", ident, valueIdent)
						helpers.Out(`return "%s"`, valueIdent)
					}
					types.Out(")")
					helpers.Out("}")
					helpers.Out(`return fmt.Sprintf("` + ident + `_%d", e)`)
					helpers.Out("}")
				}
			} else if schema.Value.Type.Is("array") {
				// do nothing
			} else {
				b, _ := schema.MarshalJSON()
				panic(fmt.Errorf("unknown schema type %s", b))
			}
			types.Out("")
		}

		if updated == 0 {
			break
		}
	}

	fmt.Println(`
package bnet

import (
"context"
"fmt"
"net/url"
)
	`)
	os.Stdout.ReadFrom(&paths)
	os.Stdout.ReadFrom(&types)
	os.Stdout.ReadFrom(&helpers)
}

func handleGenerics(schemas openapi3.Schemas) {
	for ref, schema := range schemas {
		if refToTypeOverride[ref] != "" {
			continue
		}
		if strings.Contains(ref, "SearchResultOf") {
			resultType := typeFromSchema(schema.Value.Properties["results"].Value.Items)
			if resultType == "" {
				b, _ := schema.MarshalJSON()
				panic(fmt.Errorf("result schema %s", b))
			}
			refToTypeOverride[ref] = "SearchResult[" + resultType + "]"
		} else if strings.Contains(ref, "SingleComponentResponseOf") ||
			strings.Contains(ref, "DictionaryComponentResponseOf") {
			resultType := typeFromSchema(schema.Value.Properties["data"])
			if resultType == "" {
				b, _ := schema.MarshalJSON()
				panic(fmt.Errorf("result schema %s", b))
			}
			refToTypeOverride[ref] = "ComponentResponse[" + resultType + "]"
		} else if strings.Contains(ref, "DictionaryComponentResponseOf") {
			s := schema.Value.Properties["data"]
			if v, ok := s.Value.Extensions["x-dictionary-key"]; ok {
				def, ok := v.(map[string]interface{})
				if !ok {
					b, _ := s.MarshalJSON()
					panic(fmt.Errorf("unknown dict type %s", b))
				}
				keySchema := &openapi3.Schema{}
				if format, ok := def["format"]; ok {
					keySchema.Format = format.(string)
				}
				if typ, ok := def["type"]; ok {
					keySchema.Type = &openapi3.Types{typ.(string)}
				}
				keyType := typeFromSchema(openapi3.NewSchemaRef("", keySchema))
				valueType := typeFromSchema(s.Value.AdditionalProperties.Schema)
				wantSchema[keyType] = true
				wantSchema[valueType] = true
				refToTypeOverride[ref] = "DictionaryComponentResponse[" + keyType + "," + valueType + "]"
			}
		} else if strings.Contains(ref, "Of") {
			if strings.HasPrefix(ref, "Tokens.") {
				continue
			}
			if strings.Contains(ref, "DestinyReportOffensePgcrRequest") {
				continue
			}
			b, _ := schema.MarshalJSON()
			// panic(fmt.Errorf("unknown dict type %s: %s", ref, b))
			log.Printf("potential unknown dict type %s\n\t\t\t%s", ref, b)
		}
	}
}

func checkDuplicateSchema(schemas openapi3.Schemas) {
	var found = make(map[string]string)
	for ref, _ := range schemas {
		ident := refToIdent(ref)
		if strings.HasPrefix(ident, "ItemComponentSet[") {
			continue
		}
		if found[ident] != "" {
			log.Fatalf("Duplicate schema for ident (%s): %s %s", ident, ref, found[ident])
		}
		found[ident] = ref
	}
}

func methodName(s *openapi3.PathItem) string {
	return strings.ReplaceAll(s.Summary, ".", "")
}

type buf struct {
	bytes.Buffer
}

func (b *buf) Comment(s string, params ...interface{}) {
	if len(params) > 0 {
		s = fmt.Sprintf(s, params...)
	}
	s = wordwrap.WrapString(s, 100)
	s = strings.ReplaceAll(s, "\n", "\n// ")
	b.Out("// " + s)
}

func (b *buf) Debug(s any) {
	j, _ := json.MarshalIndent(s, "// ", "  ")
	b.WriteString("// ")
	b.Write(j)
	b.WriteString("\n")
}

func (b *buf) Out(s string, params ...interface{}) {
	if len(params) == 0 {
		b.WriteString(s)
	} else {
		fmt.Fprintf(b, s, params...)
	}
	b.WriteString("\n")
}

func methodParameters(w *buf, method string, op *openapi3.Operation, paramBuf, queryBuf *buf) {
	w.Comment("%sRequest are the request parameters for operation %s", method, op.OperationID)
	w.Out(`type %sRequest struct {`, method)
	for _, param := range op.Parameters {
		b, _ := param.MarshalJSON()
		if param.Value.In != "path" && param.Value.In != "query" {
			panic(fmt.Errorf("unknown param type %s", b))
		}

		w.Out("")
		w.Comment(param.Value.Description)
		if param.Value.Required {
			w.Comment("Required.")
		}
		fieldType := typeFromSchema(param.Value.Schema)
		wantSchema[fieldType] = true
		w.Out(`%s %s`, capitalize(param.Value.Name), fieldType)
		val := fmt.Sprintf("fmt.Sprint(req.%s)", capitalize(param.Value.Name))
		if param.Value.Schema.Value.Type.Is("array") {
			val = fmt.Sprintf("joinArray(req.%s)", capitalize(param.Value.Name))
		}
		if param.Value.In == "path" {
			paramBuf.Out(`"%s":%s,`, param.Value.Name, val)
		} else if param.Value.In == "query" {
			queryBuf.Out(`"%s":{%s},`, param.Value.Name, val)
		}
	}

	if op.RequestBody != nil {
		bodySchema := op.RequestBody.Value.Content.Get("application/json").Schema
		w.Out("")
		if op.RequestBody.Value.Required {
			w.Comment("Required.")
		}
		bodyType := typeFromSchema(bodySchema)
		wantSchema[bodyType] = true
		w.Out("Body " + bodyType)
	}

	w.Out(`}`)
}

func capitalize(s string) string {
	if strings.HasSuffix(s, "Id") {
		s = strings.TrimSuffix(s, "Id")
		s += "ID"
	}
	// capitalize first letter
	return strings.ToUpper(s[:1]) + s[1:]
}

func typeFromSchema(s *openapi3.SchemaRef) (ident string) {
	defer func() {
		if override, ok := refToTypeOverride[ident]; ok {
			ident = override
		}
	}()
	if s.Ref != "" {
		return refToIdent(s.Ref)
	}
	if s.Value.Nullable {
		defer func() {
			ident = "Nullable[" + ident + "]"
		}()
	}

	if s.Value.Type.Is("object") {
		if s.Value.AllOf != nil && len(s.Value.AllOf) == 1 {
			return refToIdent(s.Value.AllOf[0].Ref)
		}

		if v, ok := s.Value.Extensions["x-dictionary-key"]; ok {
			def, ok := v.(map[string]interface{})
			if !ok {
				b, _ := s.MarshalJSON()
				panic(fmt.Errorf("unknown dict type %s", b))
			}
			keySchema := &openapi3.Schema{}
			if format, ok := def["format"]; ok {
				keySchema.Format = format.(string)
			}
			if typ, ok := def["type"]; ok {
				keySchema.Type = &openapi3.Types{typ.(string)}
			}
			keySchema.Extensions = s.Value.Extensions
			keyType := typeFromSchema(openapi3.NewSchemaRef("", keySchema))
			valueType := typeFromSchema(s.Value.AdditionalProperties.Schema)
			if enumSchema, ok := def["x-enum-reference"]; ok {
				enumSchema := enumSchema.(map[string]any)
				ref := enumSchema["$ref"].(string)
				keyType = refToIdent(ref)
				if ref == "#/components/schemas/Destiny.DestinyGender" {
					// https://github.com/Bungie-net/api/issues/1575
					keyType = "string"
				}
				if ref == "#/components/schemas/BungieCredentialType" {
					// https://github.com/Bungie-net/api/issues/1888
					keyType = "string"
				}
				if ref == "#/components/schemas/BungieMembershipType" {
					// https://github.com/Bungie-net/api/issues/1374
					keyType = "string"
				}
			}
			wantSchema[keyType] = true
			wantSchema[valueType] = true
			return "map[" + keyType + "]" + valueType
		}

		if len(s.Value.Properties) == 0 {
			return "any"
		}
	}

	if s.Value.Type.Is("array") {
		return "[]" + typeFromSchema(s.Value.Items)
	}
	if s.Value.Type.Is("string") {
		if s.Value.Format == "date-time" {
			return "Timestamp"
		}
		if s.Value.Format == "" {
			return "string"
		}
	}
	if s.Value.Type.Is("boolean") {
		return "bool"
	}

	if v, ok := s.Value.Extensions["x-enum-reference"]; ok {
		enumSchema, ok := v.(map[string]interface{})
		if !ok {
			b, _ := s.MarshalJSON()
			panic(fmt.Errorf("unknown enum type %s", b))
		}
		ref := enumSchema["$ref"].(string)
		t := refToIdent(ref)
		if v, ok := s.Value.Extensions["x-enum-is-bitmask"]; ok {
			if v.(bool) {
				return "BitmaskSet[" + t + "]"
			}
		}
		return t
	}

	if ext, ok := s.Value.Extensions["x-mapped-definition"]; ok {
		mappedTo := ext.(map[string]any)["$ref"].(string)
		ident := refToIdent(mappedTo)
		wantSchema[ident] = true
		return "Hash[" + ident + "]"
	}

	switch s.Value.Format {
	case "uint32":
		return "uint32"
	case "int32":
		return "int32"
	case "int64":
		return "Int64"
	case "byte":
		return "int"
	case "float":
		return "float64"
	case "int16":
		return "int16"
	case "double":
		return "float64"
	}

	if s.Value.Enum != nil {
		if _, ok := s.Value.Extensions["x-enum-values"]; ok {
			if s.Value.Type.Is("integer") {
				return "int"
			}
		}
	}

	b, _ := s.MarshalJSON()
	panic(fmt.Errorf("unknown type %s", b))
}

func refToIdent(ref string) (ident string) {
	defer func() {
		if override := refToTypeOverride[ref]; override != "" {
			ident = override
		}
	}()
	t := strings.TrimPrefix(ref, "#/components/schemas/")
	last := strings.LastIndex(t, ".")
	t = t[last+1:]
	t = strings.TrimPrefix(t, "Destiny2")
	t = strings.TrimPrefix(t, "Destiny")
	t = strings.TrimSuffix(t, "Enum")
	t = strings.TrimSuffix(t, "Enums")
	if strings.HasSuffix(t, "Request") {
		t = t + "Body"
	}
	wantSchema[t] = true
	return t
}

func orderedKeys[V any](m map[string]V) []string {
	var keys []string
	for k, _ := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// "[OBSOLETE]" -> Deprecated
