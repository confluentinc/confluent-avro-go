package gen_test

import (
	"bytes"
	"flag"
	"go/format"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/confluentinc/confluent-avro-go/v2"
	"github.com/confluentinc/confluent-avro-go/v2/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "Update golden files")

func TestStruct_InvalidSchemaYieldsErr(t *testing.T) {
	err := gen.Struct(`asd`, &bytes.Buffer{}, gen.Config{})

	assert.Error(t, err)
}

func TestStruct_NonRecordSchemasAreNotSupported(t *testing.T) {
	err := gen.Struct(`{"type": "string"}`, &bytes.Buffer{}, gen.Config{})

	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "only")
	assert.Contains(t, strings.ToLower(err.Error()), "record schema")
}

func TestStruct_AvroStyleCannotBeOverridden(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "test",
  "fields": [
    { "name": "someString", "type": "string" }
  ]
}`
	gc := gen.Config{
		PackageName: "Something",
		Tags: map[string]gen.TagStyle{
			"avro": gen.Kebab,
		},
	}

	_, lines := generate(t, schema, gc)

	for _, expected := range []string{
		"package something",
		"type Test struct {",
		"SomeString string `avro:\"someString\"`",
		"}",
	} {
		assert.Contains(t, lines, expected, "avro tags should not be configurable, they need to match the schema")
	}
}

func TestStruct_HandlesGoInitialisms(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "httpRecord",
  "fields": [
    { "name": "someString", "type": "string" }
  ]
}`
	gc := gen.Config{
		PackageName: "Something",
	}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "type HTTPRecord struct {")
}

func TestStruct_MultilineDoc(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Test",
  "doc": "Test record doc\nMultiline record comments",
  "fields": [
    { "name": "someString", "type": "string", "doc": "Test field doc\nMultiline field comments" }
  ]
}`
	gc := gen.Config{
		PackageName: "Something",
	}

	_, lines := generate(t, schema, gc)

	for _, expected := range []string{
		"// Test record doc",
		"// Multiline record comments.",
		"// Test field doc",
		"// Multiline field comments.",
	} {
		assert.Contains(t, lines, expected)
	}
}

func TestStruct_EscapeBacktick(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Test",
  "doc": "Test record doc with ` + "`" + `backticks` + "`" + `",
  "fields": [
    { "name": "someString", "type": "string" }
  ]
}`
	gc := gen.Config{
		PackageName: "Something",
		Encoders:    true,
		FullSchema:  true,
	}

	_, lines := generate(t, schema, gc)

	for _, expected := range []string{
		"var schemaTest = avro.MustParse(`{\"name\":\"Test\",\"doc\":\"Test record doc with ` + \"`\" + `backticks` + \"`\" + `\",\"type\":\"record\",\"fields\":[{\"name\":\"someString\",\"type\":\"string\"}]}`)",
	} {
		assert.Contains(t, lines, expected)
	}
}

func TestStruct_HandlesAdditionalInitialisms(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "CidOverHttpRecord",
  "fields": [
    { "name": "someString", "type": "string" }
  ]
}`
	gc := gen.Config{
		PackageName: "Something",
		Initialisms: []string{"CID"},
	}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "type CIDOverHTTPRecord struct {")
}

func TestStruct_HandlesStrictTypes(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "test",
  "fields": [
    { "name": "someString", "type": "int" }
  ]
}`
	gc := gen.Config{
		PackageName: "Something",
		StrictTypes: true,
	}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "SomeString int32 `avro:\"someString\"`")
}

func TestStruct_ConfigurableFieldTags(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "test",
  "fields": [
    { "name": "someSTRING", "type": "string" }
  ]
}`

	tests := []struct {
		tagStyle    gen.TagStyle
		expectedTag string
	}{
		{tagStyle: gen.Camel, expectedTag: "json:\"someString\""},
		{tagStyle: gen.Snake, expectedTag: "json:\"some_string\""},
		{tagStyle: gen.Kebab, expectedTag: "json:\"some-string\""},
		{tagStyle: gen.UpperCamel, expectedTag: "json:\"SomeString\""},
		{tagStyle: gen.Original, expectedTag: "json:\"someSTRING\""},
		{tagStyle: gen.TagStyle(""), expectedTag: "json:\"someSTRING\""},
	}

	for _, test := range tests {
		test := test
		t.Run(string(test.tagStyle), func(t *testing.T) {
			gc := gen.Config{
				PackageName: "Something",
				Tags: map[string]gen.TagStyle{
					"json": test.tagStyle,
				},
			}
			_, lines := generate(t, schema, gc)

			for _, expected := range []string{
				"package something",
				"type Test struct {",
				"SomeString string `avro:\"someSTRING\" " + test.expectedTag + "`",
				"}",
			} {
				assert.Contains(t, lines, expected)
			}
		})
	}
}

func TestStruct_ConfigurableLogicalTypes(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "test",
  "fields": [
    { "name": "id", "type": {"type": "string", "logicalType": "uuid"} }
  ]
}`

	gc := gen.Config{
		PackageName: "Something",
		LogicalTypes: []gen.LogicalType{{
			Name:             "uuid",
			Typ:              "uuid.UUID",
			ThirdPartyImport: "github.com/google/uuid",
		}},
	}
	_, lines := generate(t, schema, gc)

	for _, expected := range []string{
		"package something",
		"import (",
		"\"github.com/google/uuid\"",
		"type Test struct {",
		"ID uuid.UUID `avro:\"id\"`",
		"}",
	} {
		assert.Contains(t, lines, expected)
	}
}

func TestStruct_GenFromRecordSchema(t *testing.T) {
	fileName := "testdata/golden.go"
	gc := gen.Config{PackageName: "Something"}
	schema, err := os.ReadFile("testdata/golden.avsc")
	require.NoError(t, err)

	file, _ := generate(t, string(schema), gc)

	if *update {
		err = os.WriteFile(fileName, file, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile(fileName)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(file))
}

func TestStruct_GenFromRecordSchemaWithCustomLogicalTypes(t *testing.T) {
	fileName := "testdata/golden_logicaltype.go"

	gc := gen.Config{PackageName: "Something", LogicalTypes: []gen.LogicalType{{
		Name:             "uuid",
		Typ:              "uuid.UUID",
		ThirdPartyImport: "github.com/google/uuid",
	}}}
	schema, err := os.ReadFile("testdata/golden.avsc")
	require.NoError(t, err)

	file, _ := generate(t, string(schema), gc)

	if *update {
		err = os.WriteFile(fileName, file, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile(fileName)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(file))
}

func TestStruct_GenFromRecordSchemaWithFullName(t *testing.T) {
	schema, err := os.ReadFile("testdata/golden.avsc")
	require.NoError(t, err)

	gc := gen.Config{PackageName: "Something", FullName: true}
	file, _ := generate(t, string(schema), gc)

	if *update {
		err = os.WriteFile("testdata/golden_fullname.go", file, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile("testdata/golden_fullname.go")
	require.NoError(t, err)
	assert.Equal(t, string(want), string(file))
}

func TestStruct_GenFromRecordSchemaWithEncoders(t *testing.T) {
	schema, err := os.ReadFile("testdata/golden.avsc")
	require.NoError(t, err)

	gc := gen.Config{PackageName: "Something", Encoders: true}
	file, _ := generate(t, string(schema), gc)

	if *update {
		err = os.WriteFile("testdata/golden_encoders.go", file, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile("testdata/golden_encoders.go")
	require.NoError(t, err)
	assert.Equal(t, string(want), string(file))
}

func TestStruct_GenFromRecordSchemaWithFullSchema(t *testing.T) {
	schema, err := os.ReadFile("testdata/golden.avsc")
	require.NoError(t, err)

	gc := gen.Config{PackageName: "Something", FullSchema: true, Encoders: true}
	file, _ := generate(t, string(schema), gc)

	if *update {
		err = os.WriteFile("testdata/golden_encoders_fullschema.go", file, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile("testdata/golden_encoders_fullschema.go")
	require.NoError(t, err)
	assert.Equal(t, string(want), string(file))
}

func TestGenerator_GenEnum(t *testing.T) {
	goldenSchema, err := avro.ParseFiles("testdata/golden.avsc")
	require.NoError(t, err)

	g := gen.NewGenerator("something", map[string]gen.TagStyle{}, gen.WithEnums(true))
	g.Parse(goldenSchema)

	var buf bytes.Buffer
	err = g.Write(&buf)
	require.NoError(t, err)

	formatted, err := format.Source(buf.Bytes())
	require.NoError(t, err)

	if *update {
		err = os.WriteFile("testdata/golden_enum.go", formatted, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile("testdata/golden_enum.go")
	require.NoError(t, err)
	assert.Equal(t, string(want), string(formatted))
}

func TestGenerator(t *testing.T) {
	unionSchema, err := avro.ParseFiles("testdata/uniontype.avsc")
	require.NoError(t, err)

	mainSchema, err := avro.ParseFiles("testdata/main.avsc")
	require.NoError(t, err)

	g := gen.NewGenerator("something", map[string]gen.TagStyle{})
	g.Parse(unionSchema)
	g.Parse(mainSchema)

	var buf bytes.Buffer
	err = g.Write(&buf)
	require.NoError(t, err)

	formatted, err := format.Source(buf.Bytes())
	require.NoError(t, err)

	if *update {
		err = os.WriteFile("testdata/golden_multiple.go", formatted, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile("testdata/golden_multiple.go")
	require.NoError(t, err)
	assert.Equal(t, string(want), string(formatted))
}

func TestGenerator_CustomTemplateWithMetadata(t *testing.T) {
	unionSchema, err := avro.ParseFiles("testdata/uniontype.avsc")
	require.NoError(t, err)

	mainSchema, err := avro.ParseFiles("testdata/main.avsc")
	require.NoError(t, err)

	template := `// Code generated by avro/gen. DO NOT EDIT.
package {{ .PackageName }}

{{- range .Typedefs }}
	{{- if .Metadata }}
	// metadata: {{ .Metadata }}
	{{- end }}
	type {{ .Name }} struct {
	// fields ommitted for brevity
	}
{{- end }}`

	g := gen.NewGenerator("something", map[string]gen.TagStyle{}, gen.WithTemplate(template))
	g.ParseWithMetadata(unionSchema, "metadata for union schema")
	g.ParseWithMetadata(mainSchema, map[string]any{"metadata for": "main schema"})

	var buf bytes.Buffer
	err = g.Write(&buf)
	require.NoError(t, err)

	formatted, err := format.Source(buf.Bytes())
	require.NoError(t, err)

	if *update {
		err = os.WriteFile("testdata/golden_metadata.go", formatted, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile("testdata/golden_metadata.go")
	require.NoError(t, err)
	assert.Equal(t, string(want), string(formatted))
}

func TestTypeToFieldName(t *testing.T) {
	tests := []struct {
		typ      string
		expected string
	}{
		{"int", "Int"},
		{"int32", "Int32"},
		{"int64", "Int64"},
		{"float32", "Float32"},
		{"float64", "Float64"},
		{"bool", "Bool"},
		{"string", "String"},
		{"[]byte", "Bytes"},
		{"[]string", "StringArray"},
		{"[]int64", "Int64Array"},
		{"map[string]string", "StringMap"},
		{"map[string]int64", "Int64Map"},
		{"[7]byte", "Fixed7"},
		{"[12]byte", "Fixed12"},
		{"*big.Rat", "BigRat"},
		{"time.Time", "Time"},
		{"time.Duration", "Duration"},
		{"avro.LogicalDuration", "LogicalDuration"},
		{"SomeRecord", "SomeRecord"},
		{"[]SomeRecord", "SomeRecordArray"},
		{"*SomeRecord", "SomeRecord"},
		{"*time.Time", "Time"},
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			assert.Equal(t, tt.expected, gen.TypeToFieldName(tt.typ))
		})
	}
}

func TestStruct_UnionWrapperDisabledByDefault(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "PaymentEvent",
  "fields": [
    {
      "name": "payload",
      "type": [
        {"type": "record", "name": "CreditCard", "fields": [{"name": "number", "type": "string"}]},
        {"type": "record", "name": "BankTransfer", "fields": [{"name": "iban", "type": "string"}]}
      ]
    }
  ]
}`
	gc := gen.Config{PackageName: "Something"}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "Payload any `avro:\"payload\"`")
	assert.NotContains(t, strings.Join(lines, "\n"), "PaymentEventPayloadUnion")
}

func TestStruct_UnionWrapperNaming(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "PaymentEvent",
  "fields": [
    {
      "name": "payload",
      "type": [
        {"type": "record", "name": "CreditCard", "fields": [{"name": "number", "type": "string"}]},
        {"type": "record", "name": "BankTransfer", "fields": [{"name": "iban", "type": "string"}]}
      ]
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "Payload *PaymentEventPayloadUnion `avro:\"payload\"`")
	assert.Contains(t, lines, "type PaymentEventPayloadUnion struct {")
	assert.Contains(t, lines, "CreditCard *CreditCard")
	assert.Contains(t, lines, "BankTransfer *BankTransfer")
	assert.Contains(t, lines, "func (u *PaymentEventPayloadUnion) ToAny() (any, error) {")
	assert.Contains(t, lines, "func (u *PaymentEventPayloadUnion) FromAny(payload any) error {")
	assert.Contains(t, lines, "func (u *PaymentEventPayloadUnion) validate() error {")
	assert.Contains(t, lines, "func RegisterTypes(register func(name string, obj any)) {")
	assert.Contains(t, lines, `register("CreditCard", CreditCard{})`)
	assert.Contains(t, lines, `register("BankTransfer", BankTransfer{})`)
}

func TestStruct_UnionWrapperNullable3(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Event",
  "fields": [
    {
      "name": "data",
      "type": ["null",
        {"type": "record", "name": "TypeX", "fields": [{"name": "a", "type": "string"}]},
        {"type": "record", "name": "TypeY", "fields": [{"name": "b", "type": "int"}]}
      ],
      "default": null
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "Data *EventDataUnion `avro:\"data\"`")
	assert.Contains(t, lines, "type EventDataUnion struct {")
	assert.Contains(t, lines, "TypeX *TypeX")
	assert.Contains(t, lines, "TypeY *TypeY")
	assert.Contains(t, lines, `register("TypeX", TypeX{})`)
	assert.Contains(t, lines, `register("TypeY", TypeY{})`)
}

func TestStruct_UnionWrapperInArray(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Container",
  "fields": [
    {
      "name": "items",
      "type": {
        "type": "array",
        "items": [
          {"type": "record", "name": "TypeA", "fields": [{"name": "x", "type": "int"}]},
          {"type": "record", "name": "TypeB", "fields": [{"name": "y", "type": "string"}]}
        ]
      }
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "Items []*ContainerItemsUnion `avro:\"items\"`")
	assert.Contains(t, lines, "type ContainerItemsUnion struct {")
	assert.Contains(t, lines, "TypeA *TypeA")
	assert.Contains(t, lines, "TypeB *TypeB")
}

func TestStruct_UnionWrapperSingleMember(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Event",
  "fields": [
    {
      "name": "payload",
      "type": [
        {"type": "record", "name": "TypeA", "fields": [{"name": "x", "type": "int"}]}
      ]
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "Payload *EventPayloadUnion `avro:\"payload\"`")
	assert.Contains(t, lines, "type EventPayloadUnion struct {")
	assert.Contains(t, lines, "TypeA *TypeA")
}

func TestStruct_UnionWrapperInArraySingleMember(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Container",
  "fields": [
    {
      "name": "items",
      "type": {
        "type": "array",
        "items": [
          {"type": "record", "name": "TypeA", "fields": [{"name": "x", "type": "int"}]}
        ]
      }
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "Items []*ContainerItemsUnion `avro:\"items\"`")
	assert.Contains(t, lines, "type ContainerItemsUnion struct {")
	assert.Contains(t, lines, "TypeA *TypeA")
}

func TestStruct_UnionWrapperNullableArrayWithUnionItems(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Order",
  "fields": [
    {
      "name": "payments",
      "type": ["null", {
        "type": "array",
        "items": [
          {"type": "record", "name": "CardPayment", "fields": [{"name": "card", "type": "string"}]},
          {"type": "record", "name": "CashPayment", "fields": [{"name": "amount", "type": "int"}]}
        ]
      }],
      "default": null
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)

	assert.Contains(t, lines, "Payments *[]*OrderPaymentsUnion `avro:\"payments\"`")
	assert.Contains(t, lines, "type OrderPaymentsUnion struct {")
	assert.Contains(t, lines, "CardPayment *CardPayment")
	assert.Contains(t, lines, "CashPayment *CashPayment")
}

func TestStruct_GenFromRecordSchemaWithUnionWrappers(t *testing.T) {
	fileName := "testdata/golden_union_wrappers.go"
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}
	schema, err := os.ReadFile("testdata/golden.avsc")
	require.NoError(t, err)

	file, _ := generate(t, string(schema), gc)

	if *update {
		err = os.WriteFile(fileName, file, 0o600)
		require.NoError(t, err)
	}

	want, err := os.ReadFile(fileName)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(file))
}

func TestStruct_RegisterTypes_SkipsPrimitives(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Event",
  "fields": [
    {
      "name": "payload",
      "type": [
        "string",
        {"type": "record", "name": "TypeX", "fields": [{"name": "x", "type": "string"}]}
      ]
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)
	combined := strings.Join(lines, "\n")

	assert.Contains(t, combined, `register("TypeX", TypeX{})`)
	assert.NotContains(t, combined, `register("string"`)
}

func TestStruct_RegisterTypes_ArrayRecursion(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Container",
  "fields": [
    {
      "name": "payload",
      "type": [
        {"type": "array", "items": {"type": "record", "name": "RecordA", "fields": []}},
        {"type": "record", "name": "RecordB", "fields": []}
      ]
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)
	combined := strings.Join(lines, "\n")

	assert.Contains(t, combined, `register("array:RecordA", []RecordA{})`)
	assert.Contains(t, combined, `register("RecordB", RecordB{})`)
}

func TestStruct_RegisterTypes_MapRecursion(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Container",
  "fields": [
    {
      "name": "payload",
      "type": [
        {"type": "map", "values": {"type": "record", "name": "RecordA", "fields": []}},
        {"type": "record", "name": "RecordB", "fields": []}
      ]
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)
	combined := strings.Join(lines, "\n")

	assert.Contains(t, combined, `register("map:RecordA", map[string]RecordA{})`)
	assert.Contains(t, combined, `register("RecordB", RecordB{})`)
}

func TestStruct_RegisterTypes_Deduplication(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Event",
  "fields": [
    {
      "name": "payload1",
      "type": [
        {"type": "record", "name": "SharedRecord", "fields": []},
        {"type": "record", "name": "TypeA", "fields": []}
      ]
    },
    {
      "name": "payload2",
      "type": [
        "SharedRecord",
        {"type": "record", "name": "TypeB", "fields": []}
      ]
    }
  ]
}`
	gc := gen.Config{PackageName: "Something", UnionWrappers: true}

	_, lines := generate(t, schema, gc)
	combined := strings.Join(lines, "\n")

	assert.Equal(t, 1, strings.Count(combined, `register("SharedRecord"`), "SharedRecord should be registered exactly once")
	assert.Contains(t, combined, `register("TypeA", TypeA{})`)
	assert.Contains(t, combined, `register("TypeB", TypeB{})`)
}

func TestStruct_RegisterTypes_DisabledWithoutFlag(t *testing.T) {
	schema := `{
  "type": "record",
  "name": "Event",
  "fields": [
    {
      "name": "payload",
      "type": [
        {"type": "record", "name": "TypeX", "fields": []},
        {"type": "record", "name": "TypeY", "fields": []}
      ]
    }
  ]
}`
	gc := gen.Config{PackageName: "Something"} // UnionWrappers: false

	_, lines := generate(t, schema, gc)

	assert.NotContains(t, strings.Join(lines, "\n"), "RegisterTypes")
}

// generate is a utility to run the generation and return the result as a tuple
func generate(t *testing.T, schema string, gc gen.Config) ([]byte, []string) {
	t.Helper()

	buf := &bytes.Buffer{}
	err := gen.Struct(schema, buf, gc)
	require.NoError(t, err)

	b := make([]byte, buf.Len())
	copy(b, buf.Bytes())

	return buf.Bytes(), removeSpaceAndEmptyLines(b)
}

func removeSpaceAndEmptyLines(goCode []byte) []string {
	var lines []string
	for _, lineBytes := range bytes.Split(goCode, []byte("\n")) {
		if len(lineBytes) == 0 {
			continue
		}
		trimmed := removeMoreThanOneConsecutiveSpaces(lineBytes)
		lines = append(lines, trimmed)
	}
	return lines
}

// removeMoreThanOneConsecutiveSpaces replaces all sequences of more than one space, with a single one
func removeMoreThanOneConsecutiveSpaces(lineBytes []byte) string {
	lines := strings.TrimSpace(string(lineBytes))
	return strings.Join(regexp.MustCompile("\\s+|\\t+").Split(lines, -1), " ")
}
