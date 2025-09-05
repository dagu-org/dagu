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
