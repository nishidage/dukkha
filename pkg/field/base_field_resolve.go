package field

import (
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

func (f *BaseField) HasUnresolvedField() bool {
	return len(f.unresolvedFields) != 0
}

func (f *BaseField) ResolveFields(rc RenderingHandler, depth int, fieldName string) error {
	if atomic.LoadUint32(&f._initialized) == 0 {
		return fmt.Errorf("field resolve: struct not intialized with Init()")
	}

	if depth == 0 {
		return nil
	}

	resolveAll := len(fieldName) == 0

	parentStruct := f._parentValue.Type().Elem()
	structName := parentStruct.String()

	for i := 1; i < f._parentValue.Elem().NumField(); i++ {
		sf := parentStruct.Field(i)
		if !(sf.Name[0] >= 'A' && sf.Name[0] <= 'Z') {
			// unexported
			continue
		}

		fv := f._parentValue.Elem().Field(i)
		if !resolveAll {
			if sf.Name == fieldName {
				// this is the target field to be resolved

				return f.resolveSingleField(
					rc, depth, structName, sf.Name, fv,
				)
			}

			continue
		}

		// resolve all

		err := f.resolveSingleField(rc, depth, structName, sf.Name, fv)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *BaseField) resolveSingleField(
	rc RenderingHandler,
	depth int,

	structName string, // to make error message helpful
	fieldName string, // to make error message helpful

	targetField reflect.Value,
) error {
	handled := false
	for k, v := range f.unresolvedFields {
		if v.fieldName == fieldName {
			err := f.handleUnResolvedField(
				rc, depth, structName, fieldName, k, v, handled,
			)
			if err != nil {
				return err
			}

			handled = true
		}
	}

	return f.handleResolvedField(rc, depth, targetField)
}

// nolint:gocyclo
func (f *BaseField) handleResolvedField(
	rc RenderingHandler,
	depth int,
	targetField reflect.Value,
) error {
	if depth == 0 {
		return nil
	}

	switch targetField.Kind() {
	case reflect.Map:
		if targetField.IsNil() {
			return nil
		}

		iter := targetField.MapRange()
		for iter.Next() {
			err := f.handleResolvedField(rc, depth-1, iter.Value())
			if err != nil {
				return err
			}
		}
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		if targetField.IsNil() {
			return nil
		}

		for i := 0; i < targetField.Len(); i++ {
			tt := targetField.Index(i)
			err := f.handleResolvedField(rc, depth-1, tt)
			if err != nil {
				return err
			}
		}
	case reflect.Struct:
	case reflect.Ptr:
		fallthrough
	case reflect.Interface:
		if !targetField.IsValid() || targetField.IsZero() || targetField.IsNil() {
			return nil
		}
	default:
		// scalar types, no action required
		return nil
	}

	if targetField.CanInterface() {
		fVal, canCallResolve := targetField.Interface().(Field)
		if canCallResolve {
			return fVal.ResolveFields(rc, depth-1, "")
		}
	}

	if targetField.CanAddr() && targetField.Addr().CanInterface() {
		fVal, canCallResolve := targetField.Addr().Interface().(Field)
		if canCallResolve {
			return fVal.ResolveFields(rc, depth-1, "")
		}
	}

	return nil
}

var (
	stringPtrType         = reflect.TypeOf((*string)(nil))
	stringMapPtrType      = reflect.TypeOf((*map[string]string)(nil))
	stringBytesMapPtrType = reflect.TypeOf((*map[string][]byte)(nil))
)

func (f *BaseField) handleUnResolvedField(
	rc RenderingHandler,
	depth int,

	structName string, // to make error message helpful
	fieldName string, // to make error message helpful

	key unresolvedFieldKey,
	v *unresolvedFieldValue,
	keepOld bool,
) error {
	var target reflect.Value
	switch v.fieldValue.Kind() {
	case reflect.Ptr:
		target = v.fieldValue
	default:
		target = v.fieldValue.Addr()
	}

	for i, rawData := range v.rawData {
		toResolve := rawData
		if v.isCatchOtherField {
			toResolve = rawData.(map[string]interface{})[key.yamlKey]
		}

		var (
			resolvedValue []byte
			err           error
		)

		for _, renderer := range v.renderers {
			resolvedValue, err = rc.RenderYaml(renderer, toResolve)
			if err != nil {
				return fmt.Errorf(
					"field: failed to render value of %s.%s: %w",
					structName, fieldName, err,
				)
			}

			toResolve = resolvedValue
		}

		// if it's target is a string
		switch {
		case target.Type() == stringPtrType:
			// resolved value is the target value
			target.Elem().SetString(string(resolvedValue))
			continue
		case v.isCatchOtherField &&
			target.Type().AssignableTo(stringMapPtrType):

			target.Elem().SetMapIndex(
				reflect.ValueOf(key.yamlKey),
				reflect.ValueOf(string(resolvedValue)),
			)
			continue
		case v.isCatchOtherField &&
			target.Type().AssignableTo(stringBytesMapPtrType):
			target.Elem().SetMapIndex(
				reflect.ValueOf(key.yamlKey),
				reflect.ValueOf(resolvedValue),
			)
			continue
		}

		var tmp interface{}
		err = yaml.Unmarshal(resolvedValue, &tmp)
		if err != nil {
			return fmt.Errorf(
				"field: failed to unmarshal resolved value %q to interface: %w",
				resolvedValue, err,
			)
		}

		if v.isCatchOtherField {
			tmp = map[string]interface{}{
				key.yamlKey: tmp,
			}
		}

		// TODO: currently we alway keepOld when the filed has tag
		// 		 `dukkha:"other"`, need to ensure this behavior won't
		// 	     leave inconsistent data

		actualKeepOld := keepOld || v.isCatchOtherField || i != 0
		err = f.unmarshal(key.yamlKey, tmp, target, actualKeepOld)
		if err != nil {
			return fmt.Errorf("field: failed to unmarshal resolved value %T: %w", target, err)
		}
	}

	innerF, canCallResolve := target.Interface().(Field)
	if !canCallResolve {
		return nil
	}

	err := innerF.ResolveFields(rc, depth-1, "")
	if err != nil {
		return fmt.Errorf("failed to resolve inner field: %w", err)
	}

	return nil
}

func (f *BaseField) addUnresolvedField(
	// key part
	yamlKey string,
	suffix string,

	// value part
	fieldName string,
	fieldValue reflect.Value,
	isCatchOtherField bool,
	rawData interface{},
) error {
	if f.unresolvedFields == nil {
		f.unresolvedFields = make(map[unresolvedFieldKey]*unresolvedFieldValue)
	}

	key := unresolvedFieldKey{
		// yamlKey@suffix: ...
		yamlKey: yamlKey,
		suffix:  suffix,
	}

	oe := fieldValue
	for {
		switch oe.Kind() {
		case reflect.Slice:
			if oe.IsNil() {
				oe.Set(reflect.MakeSlice(oe.Type(), 0, 0))
			}
		case reflect.Map:
			if oe.IsNil() {
				oe.Set(reflect.MakeMap(oe.Type()))
			}
		case reflect.Interface:
			fVal, err := f.ifaceTypeHandler.Create(oe.Type(), yamlKey)
			if err != nil {
				return fmt.Errorf("failed to create interface field: %w", err)
			}

			oe.Set(reflect.ValueOf(fVal))
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

	var iface interface{}
	switch fieldValue.Kind() {
	case reflect.Ptr:
		iface = fieldValue.Interface()
	default:
		iface = fieldValue.Addr().Interface()
	}

	fVal, canCallInit := iface.(Field)
	if canCallInit {
		_ = Init(fVal, f.ifaceTypeHandler)
	}

	if isCatchOtherField {
		if f.catchOtherFields == nil {
			f.catchOtherFields = make(map[string]struct{})
		}

		f.catchOtherFields[yamlKey] = struct{}{}
	}

	if old, exists := f.unresolvedFields[key]; exists {
		old.rawData = append(old.rawData, rawData)
		old.isCatchOtherField = isCatchOtherField
		return nil
	}

	f.unresolvedFields[key] = &unresolvedFieldValue{
		fieldName:  fieldName,
		fieldValue: fieldValue,
		rawData:    []interface{}{rawData},
		renderers:  strings.Split(suffix, "|"),

		isCatchOtherField: isCatchOtherField,
	}

	return nil
}

func (f *BaseField) isCatchOtherField(yamlKey string) bool {
	if f.catchOtherFields == nil {
		return false
	}

	_, ok := f.catchOtherFields[yamlKey]
	return ok
}
