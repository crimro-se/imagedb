// querystructs is a package aiming to convert a struct of parameters into an SQL WHERE clause.
package querystructs

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
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
func BuildNullableMap(input any) (map[string]bool, error) {
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

/*
Builds a function you can use to build where clauses from structs of the same type as the one given.
Your struct needs to use three annotation tags on relevant fields:

	"ref" - name of the relevant sql column
	"db" - a placeholder name for this condition's value itself, later used by sqlx
	"clause" - there where condition clause, eg >=, =, IN, ...

	if a type is a sql.nullable, we only add it to the query string when not null
*/
func BuildWhereClauseGenerator[T any](queryStructExample T) (func(T) (string, error), error) {
	names, err := BuildAnnotationMap(queryStructExample, "db")
	if err != nil {
		return nil, err
	}
	references, err := BuildAnnotationMap(queryStructExample, "ref")
	if err != nil {
		return nil, err
	}
	if len(references) == 0 {
		return nil, errors.New("no references in provided query")
	}
	clauses, err := BuildAnnotationMap(queryStructExample, "clause")
	if err != nil {
		return nil, err
	}
	nullables, err := BuildNullableMap(queryStructExample)
	if err != nil {
		return nil, err
	}
	// non-nullable parameters are appended to prefix query
	prefixQuery := ""
	// nullable parameters are prepared for the curried function to select from
	prepared := make(map[string]string)
	for fieldName, refString := range references {
		// the db tag is used to refer to the value in this struct
		dbtag, ok := names[fieldName]
		_, isnullable := nullables[fieldName]
		if !ok {
			return nil, fmt.Errorf("missing db tag in %s", fieldName)
		}
		clause, ok := clauses[fieldName]
		if !ok {
			return nil, fmt.Errorf("missing clause tag in %s", fieldName)
		}
		switch strings.ToLower(clause) {
		case "in":
			prepared[fieldName] = fmt.Sprintf("%s IN (:%s)", refString, dbtag)
		default:
			prepared[fieldName] = fmt.Sprintf("%s %s :%s", refString, clause, dbtag)
		}
		//since the field is mandatory, we can add it to the prefix instead
		if !isnullable {
			if len(prefixQuery) > 0 {
				prefixQuery += " AND "
			}
			prefixQuery += prepared[fieldName]
			delete(prepared, fieldName)
		}
	}
	return func(queryStruct T) (string, error) {
		// local version of nullables
		nullables2, err := BuildNullableMap(queryStruct)
		suffixQuery := ""
		if err != nil {
			return "", err
		}
		for k, v := range nullables2 {
			if v {
				// this value is confirmed valid, so add it to query
				if len(suffixQuery) > 0 {
					suffixQuery += " AND "
				}
				suffixQuery += prepared[k]
			}
		}
		// unify prefix and suffix
		if len(prefixQuery) > 0 && len(suffixQuery) > 0 {
			//we need an and
			prefixQuery += " AND "
		}
		return prefixQuery + suffixQuery, nil

	}, nil
}
