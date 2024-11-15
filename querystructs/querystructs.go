// querystructs is a package aiming to convert a struct of parameters into an SQL WHERE clause.
package querystructs

import (
	"errors"
	"reflect"
)

// BuildAnnotationMap is a utility function that builds a map of [field name]=tag name
// for any given struct tag.
func BuildAnnotationMap(input any, tagname string) (map[string]string, error) {
	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, errors.New("the input needs to be struct")
	}

	annotations := make(map[string]string)
	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		tagValue, ok := field.Tag.Lookup(tagname)
		if !ok {
			continue
		}
		annotations[field.Name] = tagValue
	}
	return annotations, nil
}

// builds a map containing only Nullable fields from the input struct.
func buildNullableMap(input any) (map[string]bool, error) {
	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil, errors.New("the input needs to be struct")
	}

	nullable := make(map[string]bool)
	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Field(i)
		fieldType := val.Type().Field(i)

		// Check if the field type is a struct and has a 'Valid' field of type bool.
		if fieldVal.Kind() == reflect.Struct && fieldVal.NumField() > 0 {
			validField, found := fieldVal.Type().FieldByName("Valid")
			if found && validField.Type == reflect.TypeOf(true) {
				isValid := fieldVal.FieldByName("Valid").Bool()
				// add to our map
				nullable[fieldType.Name] = isValid
			}
		}
	}
	return nullable, nil
}
