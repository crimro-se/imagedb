package main

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// this is an enum
type SortOrder int

const (
	OrderByPathDesc SortOrder = iota
	OrderByPathAsc
	OrderByAestheticDesc
	OrderByAestheticAsc
)

// filtering criterea for retrieving images from the database
type QueryFilter struct {
	BaseDirs                   []int
	HeightMin, HeightMax       sql.NullInt64
	WidthMin, WidthMax         sql.NullInt64
	FileSizeMin, FileSizeMax   sql.NullInt64
	AestheticMin, AestheticMax sql.NullInt64
	PathStartsWith             sql.NullString
	SubPathStartsWith          sql.NullString
}

func sortOrderToQuery(so SortOrder) string {
	switch so {
	case OrderByAestheticDesc:
		return "ORDER BY aesthetic DESC"
	case OrderByAestheticAsc:
		return "ORDER BY aesthetic ASC"
	case OrderByPathDesc:
		return "ORDER BY parent_path, sub_path DESC"
	case OrderByPathAsc:
		return "ORDER BY parent_path, sub_path ASC"
	}
	return ""
}

// builds a string of the form "(col1, col2, ...) VALUES (:col1, :col2, ...)"
// based on the tagged 'db' fields in the input struct
// !panics on error
func mustStructToSQLString(input any, ignoreFields []string) string {
	val := reflect.ValueOf(input)
	if val.Kind() != reflect.Struct {
		panic(fmt.Errorf("input must be a struct"))
	}

	columns := make([]string, 0)
	values := make([]string, 0)

	// go doesn't have a Set type, so...
	ignoreSet := make(map[string]struct{})
	for _, field := range ignoreFields {
		ignoreSet[field] = struct{}{}
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		dbTag, ok := field.Tag.Lookup("db")
		if !ok || dbTag == "" {
			continue
		}
		if _, ignored := ignoreSet[dbTag]; ignored {
			continue
		}

		columns = append(columns, dbTag)
		values = append(values, fmt.Sprintf(":%s", dbTag))
	}

	columnString := strings.Join(columns, ", ")
	valueString := strings.Join(values, ", ")

	sqlString := fmt.Sprintf("(%s) VALUES (%s)", columnString, valueString)
	return sqlString
}
