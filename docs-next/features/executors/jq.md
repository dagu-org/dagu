# JQ Executor

The JQ executor allows you to process, transform, and query JSON data using the powerful `jq` command-line JSON processor.

## Overview

The JQ executor enables you to:

- Transform JSON data structures
- Extract specific fields from JSON
- Filter and query JSON data
- Format and prettify JSON output
- Perform complex JSON manipulations
- Integrate JSON processing into workflows

## Basic Usage

### Simple JSON Query

```yaml
steps:
  - name: extract-field
    executor: jq
    command: '.name'
    script: |
      {"name": "John Doe", "age": 30, "city": "New York"}
```

Output:
```json
"John Doe"
```

### Format JSON

```yaml
steps:
  - name: pretty-print
    executor: jq
    script: |
      {"id": "user123", "data": {"name": "Alice", "scores": [95, 87, 92]}}
```

Output:
```json
{
  "id": "user123",
  "data": {
    "name": "Alice",
    "scores": [
      95,
      87,
      92
    ]
  }
}
```

## JSON Transformations

### Object Transformation

```yaml
steps:
  - name: transform-object
    executor: jq
    command: '{id: .user_id, fullName: (.first_name + " " + .last_name), email: .email}'
    script: |
      {
        "user_id": 12345,
        "first_name": "John",
        "last_name": "Smith",
        "email": "john.smith@example.com",
        "created_at": "2023-01-15"
      }
```

Output:
```json
{
  "id": 12345,
  "fullName": "John Smith",
  "email": "john.smith@example.com"
}
```

### Array Processing

```yaml
steps:
  - name: process-array
    executor: jq
    command: 'map({name: .name, total: .price * .quantity})'
    script: |
      [
        {"name": "Widget A", "price": 10.99, "quantity": 5},
        {"name": "Widget B", "price": 24.99, "quantity": 2},
        {"name": "Widget C", "price": 7.50, "quantity": 10}
      ]
```

Output:
```json
[
  {"name": "Widget A", "total": 54.95},
  {"name": "Widget B", "total": 49.98},
  {"name": "Widget C", "total": 75}
]
```

### Nested Data Access

```yaml
steps:
  - name: access-nested
    executor: jq
    command: '.users[] | select(.active == true) | {id: .id, email: .contact.email}'
    script: |
      {
        "users": [
          {
            "id": 1,
            "name": "Alice",
            "active": true,
            "contact": {"email": "alice@example.com", "phone": "555-0001"}
          },
          {
            "id": 2,
            "name": "Bob",
            "active": false,
            "contact": {"email": "bob@example.com", "phone": "555-0002"}
          },
          {
            "id": 3,
            "name": "Carol",
            "active": true,
            "contact": {"email": "carol@example.com", "phone": "555-0003"}
          }
        ]
      }
```

Output:
```json
{"id": 1, "email": "alice@example.com"}
{"id": 3, "email": "carol@example.com"}
```

## Real-World Examples

### API Response Processing

```yaml
name: process-api-response
steps:
  - name: fetch-data
    executor:
      type: http
      config:
        silent: true
    command: GET https://api.example.com/products
    output: API_RESPONSE

  - name: extract-products
    executor: jq
    command: '.products | map({id: .id, name: .name, inStock: .inventory > 0})'
    script: ${API_RESPONSE}
    output: PRODUCTS
    depends: fetch-data

  - name: filter-in-stock
    executor: jq
    command: 'map(select(.inStock == true))'
    script: ${PRODUCTS}
    output: IN_STOCK_PRODUCTS
    depends: extract-products
```

### Configuration File Processing

```yaml
steps:
  - name: read-config
    command: cat /etc/myapp/config.json
    output: CONFIG

  - name: extract-database-settings
    executor: jq
    command: '.database | {connection_string: ("postgresql://\(.user):\(.password)@\(.host):\(.port)/\(.name)")}'
    script: ${CONFIG}
    output: DB_SETTINGS
    depends: read-config

  - name: use-connection
    command: |
      CONNECTION=$(echo '${DB_SETTINGS}' | jq -r '.connection_string')
      psql "$CONNECTION" -c "SELECT 1"
    depends: extract-database-settings
```

### Log Analysis

```yaml
steps:
  - name: parse-json-logs
    command: cat /var/log/app/app.log
    output: LOGS

  - name: analyze-errors
    executor: jq
    command: |
      split("\n") | 
      map(select(. != "") | fromjson) | 
      map(select(.level == "ERROR")) |
      group_by(.error_type) | 
      map({error: .[0].error_type, count: length}) |
      sort_by(.count) | reverse
    script: ${LOGS}
    output: ERROR_SUMMARY
    depends: parse-json-logs

  - name: report-top-errors
    command: |
      echo "Top Errors:"
      echo '${ERROR_SUMMARY}' | jq -r '.[] | "\(.error): \(.count) occurrences"'
    depends: analyze-errors
```

### Data Aggregation

```yaml
steps:
  - name: aggregate-sales
    executor: jq
    command: |
      group_by(.category) |
      map({
        category: .[0].category,
        total_sales: map(.amount) | add,
        item_count: length,
        avg_sale: (map(.amount) | add / length)
      })
    script: |
      [
        {"id": 1, "category": "Electronics", "amount": 299.99},
        {"id": 2, "category": "Clothing", "amount": 49.99},
        {"id": 3, "category": "Electronics", "amount": 899.99},
        {"id": 4, "category": "Clothing", "amount": 79.99},
        {"id": 5, "category": "Electronics", "amount": 149.99},
        {"id": 6, "category": "Books", "amount": 29.99}
      ]
```

Output:
```json
[
  {
    "category": "Books",
    "total_sales": 29.99,
    "item_count": 1,
    "avg_sale": 29.99
  },
  {
    "category": "Clothing",
    "total_sales": 129.98,
    "item_count": 2,
    "avg_sale": 64.99
  },
  {
    "category": "Electronics",
    "total_sales": 1349.97,
    "item_count": 3,
    "avg_sale": 449.99
  }
]
```

## Advanced Queries

### Complex Filtering

```yaml
steps:
  - name: complex-filter
    executor: jq
    command: |
      .orders |
      map(select(.status == "completed" and .total > 100)) |
      map({
        order_id: .id,
        customer: .customer.name,
        total: .total,
        items: .items | map(.name) | join(", ")
      })
    script: |
      {
        "orders": [
          {
            "id": "ORD001",
            "status": "completed",
            "total": 150.00,
            "customer": {"id": 1, "name": "Alice Johnson"},
            "items": [
              {"name": "Laptop Stand", "price": 50.00},
              {"name": "Wireless Mouse", "price": 100.00}
            ]
          },
          {
            "id": "ORD002",
            "status": "pending",
            "total": 200.00,
            "customer": {"id": 2, "name": "Bob Smith"},
            "items": [{"name": "Monitor", "price": 200.00}]
          },
          {
            "id": "ORD003",
            "status": "completed",
            "total": 75.00,
            "customer": {"id": 3, "name": "Carol White"},
            "items": [{"name": "Keyboard", "price": 75.00}]
          }
        ]
      }
```

### Recursive Descent

```yaml
steps:
  - name: find-all-emails
    executor: jq
    command: '.. | objects | select(has("email")) | .email'
    script: |
      {
        "company": {
          "name": "Tech Corp",
          "departments": [
            {
              "name": "Engineering",
              "manager": {"name": "John", "email": "john@tech.com"},
              "employees": [
                {"name": "Alice", "contact": {"email": "alice@tech.com"}},
                {"name": "Bob", "contact": {"email": "bob@tech.com"}}
              ]
            },
            {
              "name": "Sales",
              "manager": {"name": "Sarah", "email": "sarah@tech.com"},
              "employees": [
                {"name": "Dave", "contact": {"email": "dave@tech.com"}}
              ]
            }
          ]
        }
      }
```

### JSON to CSV Conversion

```yaml
steps:
  - name: json-to-csv
    executor: jq
    command: |
      ["Name","Age","City"],
      (.[] | [.name, .age, .city]) |
      @csv
    script: |
      [
        {"name": "Alice", "age": 30, "city": "New York"},
        {"name": "Bob", "age": 25, "city": "Los Angeles"},
        {"name": "Carol", "age": 35, "city": "Chicago"}
      ]
    output: CSV_DATA

  - name: save-csv
    command: echo '${CSV_DATA}' > users.csv
    depends: json-to-csv
```

## Integration with Other Steps

### Combining with HTTP Requests

```yaml
steps:
  - name: fetch-weather
    executor:
      type: http
      config:
        silent: true
    command: GET https://api.weather.com/current?city=NewYork
    output: WEATHER_DATA

  - name: extract-temperature
    executor: jq
    command: '{city: .location.city, temp_celsius: .current.temp_c, condition: .current.condition.text}'
    script: ${WEATHER_DATA}
    output: WEATHER_SUMMARY
    depends: fetch-weather

  - name: check-temperature
    command: |
      TEMP=$(echo '${WEATHER_SUMMARY}' | jq -r '.temp_celsius')
      if (( $(echo "$TEMP > 30" | bc -l) )); then
        echo "High temperature alert: ${TEMP}°C"
      fi
    depends: extract-temperature
```

### Processing Command Output

```yaml
steps:
  - name: list-processes
    command: ps aux --no-headers | awk '{print "{\"user\": \"" $1 "\", \"pid\": " $2 ", \"cpu\": " $3 ", \"mem\": " $4 ", \"command\": \"" $11 "\"}"}' | jq -s '.'
    output: PROCESSES

  - name: find-high-cpu
    executor: jq
    command: 'map(select(.cpu > 50)) | sort_by(.cpu) | reverse'
    script: ${PROCESSES}
    output: HIGH_CPU_PROCESSES
    depends: list-processes

  - name: alert-high-cpu
    command: |
      COUNT=$(echo '${HIGH_CPU_PROCESSES}' | jq 'length')
      if [ $COUNT -gt 0 ]; then
        echo "Warning: $COUNT processes using >50% CPU"
        echo '${HIGH_CPU_PROCESSES}' | jq -r '.[] | "PID \(.pid): \(.command) (\(.cpu)% CPU)"'
      fi
    depends: find-high-cpu
```

## Error Handling

### Validate JSON

```yaml
steps:
  - name: validate-json
    executor: jq
    command: '.'
    script: |
      {"valid": "json", "test": true}
    continueOn:
      failure: true
    output: VALIDATION_RESULT

  - name: handle-invalid
    command: |
      if [ -z "${VALIDATION_RESULT}" ]; then
        echo "Invalid JSON detected"
        exit 1
      fi
    depends: validate-json
```

### Safe Property Access

```yaml
steps:
  - name: safe-access
    executor: jq
    command: '.data.user // {name: "Unknown", id: 0}'
    script: |
      {"data": {}}
    output: USER_DATA
```

### Type Checking

```yaml
steps:
  - name: type-check
    executor: jq
    command: |
      if type == "array" then 
        map(select(type == "object"))
      elif type == "object" then 
        [.]
      else 
        error("Expected array or object")
      end
    script: |
      [{"id": 1}, {"id": 2}, "invalid", {"id": 3}]
```

## Performance Tips

### 1. Use Streaming for Large Files

```yaml
steps:
  - name: process-large-file
    command: |
      # Process line by line instead of loading entire file
      cat large_file.jsonl | jq -c 'select(.active == true)'
```

### 2. Combine Operations

```yaml
steps:
  - name: efficient-processing
    executor: jq
    command: |
      # Combine multiple operations in one pass
      .items |
      map(select(.price > 10 and .in_stock == true)) |
      map({id: .id, discounted_price: .price * 0.9}) |
      sort_by(.discounted_price)
    script: ${PRODUCTS}
```

### 3. Use Built-in Functions

```yaml
steps:
  - name: use-builtins
    executor: jq
    command: |
      # Use built-in functions for better performance
      {
        sum: .values | add,
        avg: .values | add / length,
        min: .values | min,
        max: .values | max
      }
    script: |
      {"values": [10, 20, 30, 40, 50]}
```

## Common Patterns

### Merging JSON Objects

```yaml
steps:
  - name: merge-configs
    executor: jq
    command: 'reduce .[] as $item ({}; . + $item)'
    script: |
      [
        {"database": {"host": "localhost"}},
        {"database": {"port": 5432}},
        {"api": {"key": "secret"}}
      ]
```

### Creating Lookup Tables

```yaml
steps:
  - name: create-lookup
    executor: jq
    command: 'map({(.id|tostring): .}) | add'
    script: |
      [
        {"id": 1, "name": "Alice", "role": "admin"},
        {"id": 2, "name": "Bob", "role": "user"},
        {"id": 3, "name": "Carol", "role": "user"}
      ]
```

### Flattening Nested Structures

```yaml
steps:
  - name: flatten-data
    executor: jq
    command: |
      .departments[] |
      .employees[] as $emp |
      {
        department: .name,
        employee_name: $emp.name,
        employee_email: $emp.email
      }
    script: |
      {
        "departments": [
          {
            "name": "Engineering",
            "employees": [
              {"name": "Alice", "email": "alice@company.com"},
              {"name": "Bob", "email": "bob@company.com"}
            ]
          },
          {
            "name": "Sales",
            "employees": [
              {"name": "Carol", "email": "carol@company.com"}
            ]
          }
        ]
      }
```

## Debugging

### Inspect Data Types

```yaml
steps:
  - name: debug-types
    executor: jq
    command: 'map({value: ., type: type})'
    script: |
      [1, "string", true, null, {"key": "value"}, [1, 2, 3]]
```

### Debug Complex Queries

```yaml
steps:
  - name: debug-step-by-step
    executor: jq
    command: |
      # Add debug output at each step
      .users |
      debug("After users:") |
      map(select(.active == true)) |
      debug("After filter:") |
      map(.email) |
      debug("Final emails:")
    script: |
      {
        "users": [
          {"name": "Alice", "email": "alice@example.com", "active": true},
          {"name": "Bob", "email": "bob@example.com", "active": false}
        ]
      }
```

## Next Steps

- Learn about [DAG Executor](/features/executors/dag) for nested workflows
- Explore [Data Flow](/features/data-flow) for managing variables
- Check out [Writing Workflows](/writing-workflows/) for more examples