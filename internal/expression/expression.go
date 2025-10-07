package expression

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/expr-lang/expr"
)

// Parse evaluates expressions in struct fields based on their tags
// Supports string fields (result converted to string), any/interface{} fields (result as-is),
// and map fields (result as-is)
func Parse[T any](ctx context.Context, input T, data any) (T, error) {
	// Create a copy of the input to avoid modifying the original
	inputValue := reflect.ValueOf(input)

	// Handle pointer types
	if inputValue.Kind() == reflect.Ptr {
		if inputValue.IsNil() {
			return input, fmt.Errorf("nil pointer passed")
		}
		inputValue = inputValue.Elem()
	}

	// Create a new instance of the same type
	resultPtr := reflect.New(inputValue.Type())
	result := resultPtr.Elem()

	// Copy the original values
	result.Set(inputValue)

	// Process the struct
	if err := processStruct(result, data); err != nil {
		return input, err
	}

	// Return the result with the same type as input
	if reflect.TypeOf(input).Kind() == reflect.Ptr {
		return resultPtr.Interface().(T), nil
	}
	return result.Interface().(T), nil
}

// processStruct recursively processes struct fields
func processStruct(structValue reflect.Value, data any) error {
	structType := structValue.Type()

	for i := 0; i < structValue.NumField(); i++ {
		field := structValue.Field(i)
		fieldType := structType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Check for expression tags
		tag := fieldType.Tag.Get("expression")

		// If field is a struct (without expression tag), process it recursively
		if field.Kind() == reflect.Struct && tag == "" {
			if err := processStruct(field, data); err != nil {
				return err
			}
			continue
		}

		// If field is a pointer to struct (without expression tag), handle it
		if field.Kind() == reflect.Ptr && field.Elem().Kind() == reflect.Struct && tag == "" {
			if !field.IsNil() {
				if err := processStruct(field.Elem(), data); err != nil {
					return err
				}
			}
			continue
		}

		// Process fields with expression tags
		if tag != "" {
			// Check if field type is supported
			if !isFieldTypeSupported(field) {
				return fmt.Errorf("expression tag can only be used on string, any/interface{}, or map fields, field %s is %s",
					fieldType.Name, field.Type())
			}

			if err := evaluateField(field, tag, data); err != nil {
				return fmt.Errorf("error evaluating field %s: %w", fieldType.Name, err)
			}
		}
	}

	return nil
}

// isFieldTypeSupported checks if the field type supports expression evaluation
func isFieldTypeSupported(field reflect.Value) bool {
	kind := field.Kind()
	return kind == reflect.String ||
		kind == reflect.Interface ||
		kind == reflect.Map
}

// evaluateField evaluates the expression for a field
func evaluateField(field reflect.Value, tagValue string, data any) error {
	// Get the field value as a string for expression evaluation
	var expression string

	switch field.Kind() {
	case reflect.String:
		expression = field.String()
	case reflect.Interface, reflect.Map:
		// For any/interface{} or map, convert the current value to string for expression
		if !field.IsNil() {
			expression = fmt.Sprintf("%v", field.Interface())
		} else {
			expression = ""
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Type())
	}

	// Evaluate based on tag type
	var result any
	var err error

	switch tagValue {
	case "template":
		// Template always returns string, but we'll convert it if needed
		resultStr, evalErr := evaluateTemplate(expression, data)
		if evalErr != nil {
			return evalErr
		}
		result = resultStr
	case "default":
		result, err = evaluateExpression(expression, data)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown expression type: %s", tagValue)
	}

	// Set the field value based on its type
	return setFieldValue(field, result)
}

// setFieldValue sets the field value based on its type
func setFieldValue(field reflect.Value, value any) error {
	switch field.Kind() {
	case reflect.String:
		// Convert result to string
		field.SetString(fmt.Sprintf("%v", value))
	case reflect.Interface:
		// Set value as-is for interface{}/any fields
		if value == nil {
			field.Set(reflect.Zero(field.Type()))
		} else {
			field.Set(reflect.ValueOf(value))
		}
	case reflect.Map:
		// For map fields, ensure the value is assignable
		if value == nil {
			field.Set(reflect.Zero(field.Type()))
		} else {
			valueReflect := reflect.ValueOf(value)
			if valueReflect.Type().AssignableTo(field.Type()) {
				field.Set(valueReflect)
			} else {
				return fmt.Errorf("cannot assign %T to %s", value, field.Type())
			}
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Type())
	}

	return nil
}

// evaluateTemplate processes template strings with embedded expressions
func evaluateTemplate(template string, data any) (string, error) {
	// Regular expression to find {{...}} patterns
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	var errors []string

	// Replace each expression with its evaluated result
	result := re.ReplaceAllStringFunc(template, func(match string) string {
		// Extract expression without the curly braces
		expr := strings.TrimSpace(match[2 : len(match)-2])

		// Evaluate the expression
		evalResult, err := evaluateExpression(expr, data)
		if err != nil {
			errors = append(errors, fmt.Sprintf("error in expression '%s': %v", expr, err))
			return match // Keep original on error
		}

		return fmt.Sprintf("%v", evalResult)
	})

	if len(errors) > 0 {
		return "", fmt.Errorf("template evaluation errors: %s", strings.Join(errors, "; "))
	}

	return result, nil
}

// evaluateExpression evaluates a single expr-lang expression
func evaluateExpression(expression string, data any) (any, error) {
	// Handle empty expression
	if strings.TrimSpace(expression) == "" {
		return nil, nil
	}

	// Prepare the environment for expr evaluation
	env := map[string]any{
		"data": data,
	}

	// If data is a map, merge it with the environment
	if dataMap, ok := data.(map[string]any); ok {
		for k, v := range dataMap {
			env[k] = v
		}
	}

	// Compile and run the expression
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("failed to compile expression: %w", err)
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("failed to run expression: %w", err)
	}

	return output, nil
}
