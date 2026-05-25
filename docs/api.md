# API Documentation

## Endpoints

### Health Check
- **GET /healthz**
  - Returns health status of the service
  - Response: `{"status":"healthy"}`

### Readiness Check
- **GET /readyz**
  - Returns readiness status of the service
  - Response: `{"status":"ready"}`

### Service Info
- **GET /info**
  - Returns service information and version
  - Response: `{"service":"goboxd","version":"0.1.0"}`

### Code Execution
- **POST /run**
  - Executes code in a sandbox
  - Request Body:
    ```json
    {
      "language": "go",
      "code": "package main\n\nimport \"fmt\"\nfunc main() { fmt.Println(\"hello\") }",
      "args": "-run -timeout 30s"
    }
    ```
  - Response:
    ```json
    {
      "job_id": "uuid",
      "status": "success|error",
      "output": "execution output",
      "error": "error message if status is error"
    }
    ```

## Status Codes

- `200 OK`: Request succeeded
- `400 Bad Request`: Invalid request format
- `404 Not Found`: Endpoint not found
- `405 Method Not Allowed`: Wrong HTTP method
- `500 Internal Server Error`: Server error
