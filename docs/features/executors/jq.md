# JQ Executor

Process and transform JSON data using jq.

## Basic Usage

```yaml
steps:
  - name: extract-field
    executor: jq
    command: '.name'
    script: |
      {"name": "John Doe", "age": 30, "city": "New York"}
```

Output: `"John Doe"`

## Examples

### Transform Objects

```yaml
steps:
  - name: transform
    executor: jq
    command: '{id: .user_id, name: (.first + " " + .last)}'
    script: |
      {"user_id": 123, "first": "John", "last": "Doe"}
```

### Filter Arrays

```yaml
steps:
  - name: filter-active
    executor: jq
    command: '.users[] | select(.active) | .email'
    script: |
      {
        "users": [
          {"email": "alice@example.com", "active": true},
          {"email": "bob@example.com", "active": false},
          {"email": "carol@example.com", "active": true}
        ]
      }
```

### Process API Response

```yaml
steps:
  - name: fetch-data
    executor:
      type: http
      config:
        silent: true
    command: GET https://api.example.com/products
    output: API_RESPONSE

  - name: extract-in-stock
    executor: jq
    command: '.products | map(select(.inventory > 0) | {id, name, price})'
    script: ${API_RESPONSE}
    output: IN_STOCK
```

### Aggregate Data

```yaml
steps:
  - name: sales-by-category
    executor: jq
    command: |
      group_by(.category) |
      map({
        category: .[0].category,
        total: map(.amount) | add,
        count: length
      })
    script: |
      [
        {"category": "Electronics", "amount": 299.99},
        {"category": "Clothing", "amount": 49.99},
        {"category": "Electronics", "amount": 199.99}
      ]
```

## Common Patterns

### Extract All Values

```yaml
steps:
  - name: find-all-emails
    executor: jq
    command: '.. | objects | select(has("email")) | .email'
    script: ${NESTED_JSON}
```

### JSON to CSV

```yaml
steps:
  - name: convert-csv
    executor: jq
    command: |
      ["Name","Age","City"],
      (.[] | [.name, .age, .city]) |
      @csv
    script: |
      [
        {"name": "Alice", "age": 30, "city": "New York"},
        {"name": "Bob", "age": 25, "city": "Los Angeles"}
      ]
    output: CSV_DATA
```

### With Shell Commands

```yaml
steps:
  - name: get-disk-usage
    command: |
      df -h | tail -n +2 | awk '{print "{\"mount\": \"" $6 "\", \"used\": \"" $5 "\"}"}' | jq -s '.'
    output: DISK_JSON

  - name: filter-high-usage
    executor: jq
    command: 'map(select(.used | rtrimstr("%") | tonumber > 80))'
    script: ${DISK_JSON}
```

### Safe Access

```yaml
steps:
  - name: safe-access
    executor: jq
    command: '.data.user // {name: "Unknown", id: 0}'
    script: ${JSON_DATA}
```

### Error Handling

```yaml
steps:
  - name: validate-json
    executor: jq
    command: 'type'
    script: ${INPUT}
    continueOn:
      failure: true
    output: JSON_TYPE
```

## Tips

- Use `-c` for compact output
- Use `-r` for raw strings (no quotes)
- Stream large files with `jq -c` line by line
- Combine operations in single pass for performance

## See Also

- [Data Flow](/features/data-flow) - Working with variables
- [Shell Executor](/features/executors/shell) - Run shell commands
