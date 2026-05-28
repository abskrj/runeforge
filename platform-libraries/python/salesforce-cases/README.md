# Salesforce Cases

Create, get, update, and delete Salesforce Cases via the REST API.

**Import**

```python
from velane import salesforce_cases
```

## Authentication

Set these as **Credentials** in the Variables tab:

| Variable | Description |
|---|---|
| `SALESFORCE_INSTANCE_URL` | Your org URL, e.g. `https://yourorg.my.salesforce.com` |
| `SALESFORCE_ACCESS_TOKEN` | Long-lived session token *(simplest option)* |

Or use the **username-password OAuth flow** instead of an access token:

| Variable | Description |
|---|---|
| `SALESFORCE_CLIENT_ID` | Connected App consumer key |
| `SALESFORCE_CLIENT_SECRET` | Connected App consumer secret |
| `SALESFORCE_USERNAME` | Salesforce username |
| `SALESFORCE_PASSWORD` | Salesforce password |
| `SALESFORCE_SECURITY_TOKEN` | Security token (appended to password if required by your org) |

## Functions

### `create_case(fields)`

Creates a new Case. Returns `{"id": ..., "success": True, "errors": []}`.

```python
result = salesforce_cases.create_case({
    "Subject": "Login not working",
    "Description": "User cannot log in after password reset.",
    "Status": "New",
    "Priority": "High",
    "Origin": "Web",
})
print(result["id"])  # 5001000000D8cuBAAR
```

### `get_case(case_id)`

Fetches a Case by its 15- or 18-character Salesforce ID.

```python
case = salesforce_cases.get_case("5001000000D8cuBAAR")
print(case["Subject"], case["Status"])
```

### `update_case(case_id, fields)`

Updates an existing Case. Only the provided fields are changed.

```python
salesforce_cases.update_case("5001000000D8cuBAAR", {"Status": "Closed"})
```

### `delete_case(case_id)`

Permanently deletes a Case.

```python
salesforce_cases.delete_case("5001000000D8cuBAAR")
```

## Common Field Values

| Field | Values |
|---|---|
| `Status` | `New`, `Working`, `Escalated`, `Closed` |
| `Priority` | `High`, `Medium`, `Low` |
| `Origin` | `Email`, `Phone`, `Web` |
