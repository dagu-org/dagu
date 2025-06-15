# HTTP Executor

The HTTP executor enables you to make HTTP requests as part of your workflows, perfect for integrating with web services, REST APIs, and webhooks.

## Overview

The HTTP executor allows you to:

- Make requests with any HTTP method (GET, POST, PUT, DELETE, etc.)
- Set custom headers and authentication
- Send request bodies in various formats
- Handle query parameters
- Control timeouts and response handling
- Process API responses in your workflow

## Basic Usage

### Simple GET Request

```yaml
steps:
  - name: get-data
    executor:
      type: http
    command: GET https://api.example.com/data
```

### POST Request with Body

```yaml
steps:
  - name: create-resource
    executor:
      type: http
      config:
        body: '{"name": "New Resource", "type": "example"}'
        headers:
          Content-Type: application/json
    command: POST https://api.example.com/resources
```

## Request Configuration

### HTTP Methods

All standard HTTP methods are supported:

```yaml
steps:
  - name: get-request
    executor: http
    command: GET https://api.example.com/users

  - name: post-request
    executor: http
    command: POST https://api.example.com/users

  - name: put-request
    executor: http
    command: PUT https://api.example.com/users/123

  - name: delete-request
    executor: http
    command: DELETE https://api.example.com/users/123

  - name: patch-request
    executor: http
    command: PATCH https://api.example.com/users/123

  - name: head-request
    executor: http
    command: HEAD https://api.example.com/status
```

### Headers

Set custom headers for your requests:

```yaml
steps:
  - name: with-headers
    executor:
      type: http
      config:
        headers:
          Authorization: "Bearer ${API_TOKEN}"
          Content-Type: "application/json"
          Accept: "application/json"
          X-Custom-Header: "custom-value"
    command: GET https://api.example.com/protected
```

### Query Parameters

Add query parameters to your requests:

```yaml
steps:
  - name: with-query-params
    executor:
      type: http
      config:
        query:
          page: "1"
          limit: "50"
          sort: "created_at"
          filter: "active"
    command: GET https://api.example.com/items
    # Results in: https://api.example.com/items?page=1&limit=50&sort=created_at&filter=active
```

### Request Body

Send data in the request body:

```yaml
steps:
  - name: json-body
    executor:
      type: http
      config:
        body: |
          {
            "username": "john_doe",
            "email": "john@example.com",
            "roles": ["user", "admin"]
          }
        headers:
          Content-Type: application/json
    command: POST https://api.example.com/users

  - name: form-data
    executor:
      type: http
      config:
        body: "username=john_doe&email=john@example.com"
        headers:
          Content-Type: application/x-www-form-urlencoded
    command: POST https://api.example.com/login

  - name: plain-text
    executor:
      type: http
      config:
        body: "This is plain text content"
        headers:
          Content-Type: text/plain
    command: POST https://api.example.com/messages
```

## Response Handling

### Silent Mode

Control output verbosity:

```yaml
steps:
  - name: verbose-response
    executor:
      type: http
      config:
        silent: false  # Default - shows headers and body
    command: GET https://api.example.com/data

  - name: body-only
    executor:
      type: http
      config:
        silent: true  # Only outputs response body
    command: GET https://api.example.com/data
    output: API_DATA
```

### Capture Response

Store API responses for later use:

```yaml
steps:
  - name: get-user
    executor:
      type: http
      config:
        silent: true
        headers:
          Accept: application/json
    command: GET https://api.example.com/user/profile
    output: USER_PROFILE

  - name: process-user
    command: echo "User email is ${USER_PROFILE.email}"
    depends: get-user
```

### Timeout Configuration

Set request timeouts:

```yaml
steps:
  - name: with-timeout
    executor:
      type: http
      config:
        timeout: 30  # 30 second timeout
        headers:
          Accept: application/json
    command: GET https://slow-api.example.com/data
```

## Authentication Examples

### Bearer Token

```yaml
env:
  - API_TOKEN: ${API_TOKEN}

steps:
  - name: bearer-auth
    executor:
      type: http
      config:
        headers:
          Authorization: "Bearer ${API_TOKEN}"
    command: GET https://api.example.com/protected
```

### Basic Authentication

```yaml
env:
  - AUTH_USER: myuser
  - AUTH_PASS: mypassword

steps:
  - name: basic-auth
    executor:
      type: http
      config:
        headers:
          Authorization: "Basic ${AUTH_USER}:${AUTH_PASS}"
    command: GET https://api.example.com/secure
```

### API Key Authentication

```yaml
steps:
  - name: api-key-header
    executor:
      type: http
      config:
        headers:
          X-API-Key: "${API_KEY}"
    command: GET https://api.example.com/data

  - name: api-key-query
    executor:
      type: http
      config:
        query:
          api_key: "${API_KEY}"
    command: GET https://api.example.com/data
```

## Real-World Examples

### REST API CRUD Operations

```yaml
name: user-management
env:
  - API_BASE: https://api.example.com
  - API_TOKEN: ${API_TOKEN}

steps:
  # Create
  - name: create-user
    executor:
      type: http
      config:
        body: |
          {
            "name": "John Doe",
            "email": "john@example.com",
            "role": "user"
          }
        headers:
          Authorization: "Bearer ${API_TOKEN}"
          Content-Type: application/json
        silent: true
    command: POST ${API_BASE}/users
    output: NEW_USER

  # Read
  - name: get-user
    executor:
      type: http
      config:
        headers:
          Authorization: "Bearer ${API_TOKEN}"
        silent: true
    command: GET ${API_BASE}/users/${NEW_USER.id}
    depends: create-user

  # Update
  - name: update-user
    executor:
      type: http
      config:
        body: '{"role": "admin"}'
        headers:
          Authorization: "Bearer ${API_TOKEN}"
          Content-Type: application/json
    command: PATCH ${API_BASE}/users/${NEW_USER.id}
    depends: get-user

  # Delete
  - name: delete-user
    executor:
      type: http
      config:
        headers:
          Authorization: "Bearer ${API_TOKEN}"
    command: DELETE ${API_BASE}/users/${NEW_USER.id}
    depends: update-user
```

### Webhook Integration

```yaml
name: deployment-webhook
params:
  - ENVIRONMENT: staging
  - VERSION: 1.2.3

steps:
  - name: notify-start
    executor:
      type: http
      config:
        body: |
          {
            "text": "Deployment started",
            "environment": "${ENVIRONMENT}",
            "version": "${VERSION}",
            "timestamp": "`date -u +%Y-%m-%dT%H:%M:%SZ`"
          }
        headers:
          Content-Type: application/json
    command: POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL

  - name: deploy
    command: ./deploy.sh ${ENVIRONMENT} ${VERSION}
    depends: notify-start

  - name: notify-complete
    executor:
      type: http
      config:
        body: |
          {
            "text": "Deployment completed",
            "environment": "${ENVIRONMENT}",
            "version": "${VERSION}",
            "status": "success"
          }
        headers:
          Content-Type: application/json
    command: POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL
    depends: deploy
```

### API Polling

```yaml
name: wait-for-job
steps:
  - name: start-job
    executor:
      type: http
      config:
        body: '{"task": "process-data"}'
        headers:
          Content-Type: application/json
        silent: true
    command: POST https://api.example.com/jobs
    output: JOB

  - name: poll-status
    executor:
      type: http
      config:
        headers:
          Accept: application/json
        silent: true
        timeout: 10
    command: GET https://api.example.com/jobs/${JOB.id}
    output: STATUS
    retryPolicy:
      limit: 30
      intervalSec: 10
    continueOn:
      output: ["PENDING", "RUNNING"]
    depends: start-job

  - name: get-results
    executor:
      type: http
      config:
        silent: true
    command: GET https://api.example.com/jobs/${JOB.id}/results
    depends: poll-status
```

### Multi-Service Integration

```yaml
name: order-processing
steps:
  # Check inventory
  - name: check-inventory
    executor:
      type: http
      config:
        silent: true
        headers:
          X-API-Key: ${INVENTORY_API_KEY}
    command: GET https://inventory.example.com/api/products/12345
    output: INVENTORY

  # Create order
  - name: create-order
    executor:
      type: http
      config:
        body: |
          {
            "product_id": "12345",
            "quantity": 2,
            "customer_id": "cust-789"
          }
        headers:
          Authorization: "Bearer ${ORDER_API_TOKEN}"
          Content-Type: application/json
        silent: true
    command: POST https://orders.example.com/api/orders
    output: ORDER
    preconditions:
      - condition: "${INVENTORY.available}"
        expected: "true"
    depends: check-inventory

  # Process payment
  - name: charge-payment
    executor:
      type: http
      config:
        body: |
          {
            "order_id": "${ORDER.id}",
            "amount": "${ORDER.total}",
            "currency": "USD"
          }
        headers:
          Authorization: "Bearer ${PAYMENT_API_TOKEN}"
          Content-Type: application/json
    command: POST https://payments.example.com/api/charges
    depends: create-order

  # Send notification
  - name: notify-customer
    executor:
      type: http
      config:
        body: |
          {
            "to": "${ORDER.customer_email}",
            "template": "order_confirmation",
            "data": {
              "order_id": "${ORDER.id}",
              "total": "${ORDER.total}"
            }
          }
        headers:
          X-API-Key: ${EMAIL_API_KEY}
          Content-Type: application/json
    command: POST https://email.example.com/api/send
    depends: charge-payment
```

## Error Handling

### HTTP Status Codes

Handle different response codes:

```yaml
steps:
  - name: handle-status
    executor:
      type: http
      config:
        silent: true
    command: GET https://api.example.com/data
    continueOn:
      exitCode: [1]  # Non-2xx status codes return exit code 1
    output: RESPONSE

  - name: check-response
    command: |
      if [ $? -eq 0 ]; then
        echo "Success: ${RESPONSE}"
      else
        echo "Failed with status code"
      fi
    depends: handle-status
```

### Retry on Failure

Implement retry logic for transient failures:

```yaml
steps:
  - name: resilient-request
    executor:
      type: http
      config:
        timeout: 30
        headers:
          Accept: application/json
    command: GET https://unreliable-api.example.com/data
    retryPolicy:
      limit: 3
      intervalSec: 5
      exponentialBackoff: true
```

### Fallback Handling

```yaml
steps:
  - name: primary-api
    executor:
      type: http
      config:
        timeout: 10
        silent: true
    command: GET https://primary-api.example.com/data
    continueOn:
      failure: true
    output: PRIMARY_RESULT

  - name: fallback-api
    executor:
      type: http
      config:
        timeout: 10
        silent: true
    command: GET https://backup-api.example.com/data
    preconditions:
      - condition: "${PRIMARY_RESULT}"
        expected: ""
    depends: primary-api
```

## Best Practices

### 1. Use Environment Variables for Sensitive Data

```yaml
# Good - secure
env:
  - API_TOKEN: ${API_TOKEN}  # From environment

steps:
  - name: secure-request
    executor:
      type: http
      config:
        headers:
          Authorization: "Bearer ${API_TOKEN}"
    command: GET https://api.example.com/data

# Bad - hardcoded
steps:
  - name: insecure-request
    executor:
      type: http
      config:
        headers:
          Authorization: "Bearer hardcoded-token-here"
    command: GET https://api.example.com/data
```

### 2. Set Appropriate Timeouts

```yaml
steps:
  - name: quick-endpoint
    executor:
      type: http
      config:
        timeout: 5  # Fast endpoint
    command: GET https://api.example.com/health

  - name: slow-endpoint
    executor:
      type: http
      config:
        timeout: 300  # Long-running operation
    command: POST https://api.example.com/process-large-dataset
```

### 3. Use Silent Mode for Clean Output

```yaml
steps:
  - name: get-json
    executor:
      type: http
      config:
        silent: true  # Only return body for parsing
        headers:
          Accept: application/json
    command: GET https://api.example.com/data
    output: JSON_DATA

  - name: parse-json
    command: echo "${JSON_DATA}" | jq '.items | length'
    depends: get-json
```

### 4. Handle Pagination

```yaml
name: fetch-all-pages
env:
  - API_URL: https://api.example.com/items
  - PAGE: 1

steps:
  - name: fetch-page
    executor:
      type: http
      config:
        silent: true
        query:
          page: "${PAGE}"
          limit: "100"
    command: GET ${API_URL}
    output: PAGE_DATA

  - name: process-page
    command: |
      echo "${PAGE_DATA}" | jq '.items[]' >> all_items.json
      
      # Check if there are more pages
      if [ $(echo "${PAGE_DATA}" | jq '.has_next') = "true" ]; then
        echo "$((PAGE + 1))" > next_page.txt
      fi
    depends: fetch-page

  - name: fetch-next
    run: self  # Recursive call
    params: "PAGE=`cat next_page.txt 2>/dev/null || echo 0`"
    preconditions:
      - condition: "`cat next_page.txt 2>/dev/null || echo 0`"
        expected: "!0"
    depends: process-page
```

## Debugging

### Verbose Output

```yaml
steps:
  - name: debug-request
    executor:
      type: http
      config:
        silent: false  # Show full request/response details
    command: GET https://api.example.com/debug
```

### Log Headers and Body

```yaml
steps:
  - name: detailed-debug
    command: |
      # Use curl for detailed debugging
      curl -v \
        -H "Authorization: Bearer ${API_TOKEN}" \
        -H "Content-Type: application/json" \
        -d '{"test": true}' \
        https://api.example.com/test
```

### Test Connectivity

```yaml
steps:
  - name: test-connection
    executor:
      type: http
      config:
        timeout: 5
    command: HEAD https://api.example.com/health
    continueOn:
      failure: true

  - name: report-status
    command: |
      if [ $? -eq 0 ]; then
        echo "API is reachable"
      else
        echo "API is unreachable"
      fi
    depends: test-connection
```

## Next Steps

- Learn about [SSH Executor](/features/executors/ssh) for remote command execution
- Explore [Mail Executor](/features/executors/mail) for sending notifications
- Check out [Data Flow](/features/data-flow) for handling API responses