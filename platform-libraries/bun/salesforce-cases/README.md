# Salesforce Cases

Create, get, update, and delete Salesforce Cases via the REST API.

**Import**

```ts
import { createCase, getCase, updateCase, deleteCase } from '@velane/salesforce-cases'
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
| `SALESFORCE_SECURITY_TOKEN` | Security token (append to password if required by your org) |

## Functions

### `createCase(fields)`

Creates a new Case. Returns `{ id, success, errors }`.

```ts
const result = await createCase({
  Subject: 'Login not working',
  Description: 'User cannot log in after password reset.',
  Status: 'New',
  Priority: 'High',
  Origin: 'Web',
})
console.log(result.id) // 5001000000D8cuBAAR
```

### `getCase(id)`

Fetches a Case by its 15- or 18-character Salesforce ID.

```ts
const c = await getCase('5001000000D8cuBAAR')
console.log(c.Subject, c.Status)
```

### `updateCase(id, fields)`

Updates an existing Case. Only the provided fields are changed.

```ts
await updateCase('5001000000D8cuBAAR', { Status: 'Closed' })
```

### `deleteCase(id)`

Permanently deletes a Case.

```ts
await deleteCase('5001000000D8cuBAAR')
```

## Common Field Values

| Field | Values |
|---|---|
| `Status` | `New`, `Working`, `Escalated`, `Closed` |
| `Priority` | `High`, `Medium`, `Low` |
| `Origin` | `Email`, `Phone`, `Web` |
