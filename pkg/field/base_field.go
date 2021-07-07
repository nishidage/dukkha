package field

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"

	"arhat.dev/pkg/log"
	"gopkg.in/yaml.v3"

	"arhat.dev/dukkha/pkg/types"
)

type (
	unresolvedFieldKey struct {
		fieldName string

		// TODO: put renderer in key is to support fields with
		// 		`dukkha:"other"` field tag, each item should be able
		// 		to have its own renderer
		// 		but the `other` fields get no rendering suffix support
		// 		for the time being
		//
		// 		so we just keep it here for future
		renderer string
	}

	unresolvedFieldValue struct {
		fieldValue    reflect.Value
		yamlFieldName string
		rawData       []interface{}
	}
)

var (
	stringPtrType = reflect.TypeOf((*string)(nil))
)

type BaseField struct {
	_initialized uint32

	// _parentValue is always a pointer type with .Elem() to the struct
	// when initialized
	_parentValue reflect.Value

	unresolvedFields map[unresolvedFieldKey]*unresolvedFieldValue
}

// UnmarshalYAML handles renderer suffix
// nolint:gocyclo,revive
func (self *BaseField) UnmarshalYAML(n *yaml.Node) error {
	if atomic.LoadUint32(&self._initialized) == 0 {
		return fmt.Errorf("field unmarshal: struct not intialized with Init()")
	}

	type fieldKey struct {
		yamlKey string
	}

	type fieldSpec struct {
		fieldName  string
		fieldValue reflect.Value
		base       *BaseField
	}

	fields := make(map[fieldKey]*fieldSpec)
	pt := self._parentValue.Type().Elem()

	addField := func(
		yamlKey, fieldName string,
		fieldValue reflect.Value,
		base *BaseField,
	) bool {
		key := fieldKey{yamlKey: yamlKey}
		if _, exists := fields[key]; exists {
			return false
		}

		fields[key] = &fieldSpec{
			fieldName: fieldName,

			fieldValue: fieldValue,
			base:       base,
		}
		return true
	}

	getField := func(yamlKey string) *fieldSpec {
		return fields[fieldKey{
			yamlKey: yamlKey,
		}]
	}

	logger := log.Log.WithName("BaseField").WithFields(
		log.String("func", "UnmarshalYAML"),
		log.String("struct", pt.String()),
	)

	var catchOtherField *fieldSpec
	// get expected fields first, the first field (myself)
fieldLoop:
	for i := 1; i < pt.NumField(); i++ {
		fieldType := pt.Field(i)
		fieldValue := self._parentValue.Elem().Field(i)

		// initialize struct fields accepted by Init(), in case being used later
		initAllStructCanCallInit(fieldValue)

		yTags := strings.Split(fieldType.Tag.Get("yaml"), ",")

		// check if ignored
		for _, t := range yTags {
			if t == "-" {
				// ignored
				continue fieldLoop
			}
		}

		// get yaml field name
		yamlKey := yTags[0]
		if len(yamlKey) != 0 {
			if !addField(yamlKey, fieldType.Name, fieldValue, self) {
				return fmt.Errorf(
					"field: duplicate yaml key %q in %s",
					yamlKey, pt.String(),
				)
			}
		}

		// process yaml tag flags
		for _, t := range yTags[1:] {
			switch t {
			case "inline":
				kind := fieldType.Type.Kind()
				switch {
				case kind == reflect.Struct:
				case kind == reflect.Ptr && fieldType.Type.Elem().Kind() == reflect.Struct:
				default:
					return fmt.Errorf(
						"field: non struct nor struct pointer field %s.%s has inline tag",
						pt.String(), fieldType.Name,
					)
				}

				logger.V("inspecting inline field", log.String("name", fieldType.Name))

				inlineFv := fieldValue
				inlineFt := self._parentValue.Type().Elem().Field(i).Type

				var iface interface{}
				switch inlineFv.Kind() {
				case reflect.Ptr:
					iface = inlineFv.Interface()
				default:
					iface = inlineFv.Addr().Interface()
				}

				base := self
				fVal, canCallInit := iface.(types.Field)
				if canCallInit {
					innerBaseF := reflect.ValueOf(Init(fVal)).Elem().Field(0)

					if innerBaseF.Kind() == reflect.Struct {
						if innerBaseF.Addr().Type() == baseFieldPtrType {
							base = innerBaseF.Addr().Interface().(*BaseField)
						}
					} else {
						if innerBaseF.Type() == baseFieldPtrType {
							base = innerBaseF.Interface().(*BaseField)
						}
					}
				}

				for j := 0; j < inlineFv.NumField(); j++ {
					innerFv := inlineFv.Field(j)
					innerFt := inlineFt.Field(j)

					innerYamlKey := strings.Split(innerFt.Tag.Get("yaml"), ",")[0]
					if innerYamlKey == "-" {
						continue
					}

					if len(innerYamlKey) == 0 {
						// already in a inline field, do not check inline anymore
						continue
					}

					if !addField(innerYamlKey, innerFt.Name, innerFv, base) {
						return fmt.Errorf(
							"field: duplicate yaml key %q in inline field %s of %s",
							innerYamlKey, innerFt.Name, pt.String(),
						)
					}
				}
			default:
				// TODO: handle other yaml tag flags
			}
		}

		// dukkha tag is used to extend yaml tag
		dTags := strings.Split(fieldType.Tag.Get("dukkha"), ",")
		for _, t := range dTags {
			switch t {
			case "other":
				// other is used to match unhandled values
				// only supports map[string]Any

				if catchOtherField != nil {
					return fmt.Errorf(
						"field: bad field tags in %s: only one map in a struct can have `dukkha:\"other\"` tag",
						pt.String(),
					)
				}

				logger.V("found catch other field", log.String("field", fieldType.Name))
				catchOtherField = &fieldSpec{
					fieldName:  fieldType.Name,
					fieldValue: fieldValue,
					base:       self,
				}
			case "":
			default:
				return fmt.Errorf("field: unknown dukkha tag value %q", t)
			}
		}
	}

	switch n.ShortTag() {
	case "!!map":
	default:
		return fmt.Errorf("field: unsupported yaml tag %q when handling %s", n.Tag, pt.String())
	}

	dataBytes, err := yaml.Marshal(n)
	if err != nil {
		return fmt.Errorf("field: data marshal back failed for %s: %w", pt.String(), err)
	}

	m := make(map[string]interface{})
	err = yaml.Unmarshal(dataBytes, &m)
	if err != nil {
		return fmt.Errorf("field: data unmarshal failed for %s: %w", pt.String(), err)
	}

	handledYamlValues := make(map[string]struct{})
	// handle rendering suffix
	for rawYamlKey, v := range m {
		yamlKey := rawYamlKey

		logger := logger.WithFields(log.String("raw_yaml_field", rawYamlKey))

		logger.V("inspecting yaml field")

		parts := strings.SplitN(rawYamlKey, "@", 2)
		if len(parts) == 1 {
			// no rendering suffix, fill value

			if _, ok := handledYamlValues[yamlKey]; ok {
				return fmt.Errorf(
					"field: duplicate yaml field name %q",
					yamlKey,
				)
			}

			handledYamlValues[yamlKey] = struct{}{}

			fSpec := getField(yamlKey)
			if fSpec == nil {
				if catchOtherField == nil {
					return fmt.Errorf("field: unknown yaml field %q for %s", yamlKey, pt.String())
				}

				fSpec = catchOtherField
				v = map[string]interface{}{
					yamlKey: v,
				}
			}

			logger := logger.WithFields(log.String("field", fSpec.fieldName))

			logger.V("working on plain field")

			err = unmarshal(yamlKey, v, fSpec.fieldValue, true)
			if err != nil {
				return fmt.Errorf(
					"field: failed to unmarshal yaml field %q to struct field %q: %w",
					yamlKey, fSpec.fieldName, err,
				)
			}

			continue
		}

		// has rendering suffix

		yamlKey, renderer := parts[0], parts[1]

		logger = logger.WithFields(
			log.String("yaml_field", yamlKey),
			log.String("renderer", renderer),
		)

		if _, ok := handledYamlValues[yamlKey]; ok {
			return fmt.Errorf(
				"field: duplicate yaml field name %q, rendering suffix won't change the field name",
				yamlKey,
			)
		}

		fSpec := getField(yamlKey)
		if fSpec == nil {
			if catchOtherField == nil {
				return fmt.Errorf("field: unknown yaml field %q for %s", yamlKey, pt.String())
			}

			fSpec = catchOtherField
		}

		logger = logger.WithFields(log.String("field", fSpec.fieldName))

		// do not unmarshal now, we need to evaluate value and unmarshal
		//
		// 		err = unmarshal(v, fSpec.fieldValue)
		//

		handledYamlValues[yamlKey] = struct{}{}
		// don't forget the raw name with rendering suffix
		handledYamlValues[rawYamlKey] = struct{}{}

		logger.V("found field to be rendered")

		err = fSpec.base.addUnresolvedField(
			fSpec.fieldName, fSpec.fieldValue,
			yamlKey,
			renderer,
			m[rawYamlKey],
		)
		if err != nil {
			return fmt.Errorf("field: failed to add unresolved field: %w", err)
		}
	}

	for k := range handledYamlValues {
		delete(m, k)
	}

	if len(m) == 0 {
		// all values consumed
		return nil
	}

	var unknownFields []string
	for k := range m {
		unknownFields = append(unknownFields, k)
	}

	sort.Strings(unknownFields)

	return fmt.Errorf(
		"field: unknown yaml fields for %s: %s",
		pt.String(), strings.Join(unknownFields, ", "),
	)
}

func initAllStructCanCallInit(fieldValue reflect.Value) {
	if fieldValue.Kind() != reflect.Struct {
		return
	}

	if fieldValue.Type() == baseFieldStructType {
		return
	}

	if !fieldValue.CanAddr() {
		return
	}

	if !fieldValue.Addr().CanInterface() {
		return
	}

	iface, canCallInit := fieldValue.Addr().Interface().(types.Field)
	if canCallInit {
		_ = Init(iface)
	}

	for i := 0; i < fieldValue.NumField(); i++ {
		initAllStructCanCallInit(fieldValue.Field(i))
	}
}

func unmarshal(yamlKey string, in interface{}, outField reflect.Value, keepOld bool) error {
	oe := outField

	for {
		switch oe.Kind() {
		case reflect.Slice:
			switch in.(type) {
			case []interface{}:
			case nil:
				return nil
			default:
				return fmt.Errorf("unexpected non slice data for %q", outField.String())
			}

			inSlice := in.([]interface{})
			size := len(inSlice)

			sliceVal := reflect.MakeSlice(oe.Type(), size, size)

			for i := 0; i < size; i++ {
				itemVal := sliceVal.Index(i)

				err := unmarshal(yamlKey, inSlice[i], itemVal, keepOld)
				if err != nil {
					return fmt.Errorf("failed to unmarshal slice item %s: %w", itemVal.Type().String(), err)
				}
			}

			if oe.IsZero() || !keepOld {
				oe.Set(sliceVal)
			} else {
				oe.Set(reflect.AppendSlice(oe, sliceVal))
			}

			return nil
		case reflect.Map:
			// map key MUST be string
			if oe.IsZero() || !keepOld {
				oe.Set(reflect.MakeMap(oe.Type()))
			}

			valType := oe.Type().Elem()

			iter := reflect.ValueOf(in).MapRange()
			for iter.Next() {
				valVal := reflect.New(valType)
				err := unmarshal(
					iter.Key().String(),
					iter.Value().Interface(),
					valVal,
					keepOld,
				)
				if err != nil {
					return fmt.Errorf("failed to unmarshal map value %s for key %q: %w",
						valType.String(), iter.Key().String(), err,
					)
				}

				oe.SetMapIndex(iter.Key(), valVal.Elem())
			}

			return nil
		case reflect.Interface:
			fVal, err := CreateInterfaceField(oe.Type(), yamlKey)
			if err != nil {
				return fmt.Errorf("failed to create interface field: %w", err)
			}

			val := reflect.ValueOf(fVal)
			outField.Set(val)

			// DO NOT use outField directly, which will always match reflect.Interface
			return unmarshal(yamlKey, in, val, keepOld)
		case reflect.Ptr:
			// process later
		default:
			// scalar types or struct/array/func/chan/unsafe.Pointer
			// hand it to go-yaml
		}

		if oe.Kind() != reflect.Ptr {
			break
		}

		if oe.IsZero() {
			oe.Set(reflect.New(oe.Type().Elem()))
		}

		oe = oe.Elem()
	}

	var out interface{}
	if outField.Kind() != reflect.Ptr {
		out = outField.Addr().Interface()
	} else {
		out = outField.Interface()
	}

	fVal, canCallInit := out.(types.Field)
	if canCallInit {
		_ = Init(fVal)
	}

	dataBytes, err := yaml.Marshal(in)
	if err != nil {
		return fmt.Errorf("field: failed to marshal back yaml field %q: %w", yamlKey, err)
	}

	return yaml.Unmarshal(dataBytes, out)
}
