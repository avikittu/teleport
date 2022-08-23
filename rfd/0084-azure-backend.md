---
authors: Edoardo Spadolini (edoardo.spadolini@goteleport.com)
state: draft
---

# RFD 0084 - Azure backend storage

## Required Approvers
* Engineering: TBD
* Security: TBD
* Product: TBD

## What

Add support for Azure IAM authentication to the Postgres backend, add audit log capabilities to the `sqlbk`/Postgres backend, add a session uploader that uses Azure Blob Storage (with the same IAM authentication).

## Why

To support deployments of Teleport in Azure with the same convenience and reliability that's currently available when using managed storage in AWS and GCP.

## Details

### Auth backend

Even though our backend is ultimately just a key-value store, the managed NoSQL database offering in Azure, called Cosmos DB, is not sufficient for our purpose, because our cache propagation model relies on all auth servers having access to a stream of events that replicates all the changes applied to the backend. Cosmos DB has a change feed, but it doesn't contain every change to values and it doesn't have deletions - as such, it's not usable as the backing store for our auth backend (a "full fidelity" change feed has been [in private preview since 2020](https://azure.microsoft.com/en-us/updates/change-feed-with-full-database-operations-for-azure-cosmos-db/)).

As using Postgres as the backend storage for Teleport has been possible (in Preview) since v9.0.4, and Azure offers a managed Postgres service, we will be adding support for Azure AD authentication to the Postgres backend - that's the only change needed to support Azure Postgres in Teleport. As of August 2022, only the "Single Server" flavor of Azure Postgres supports AD authentication and it's limited to Postgres 11, which will be [deprecated in November 2023](https://docs.microsoft.com/en-us/azure/postgresql/single-server/concepts-version-policy#major-version-retirement-policy), but it seems likely that AD authentication will also be supported in the "Flexible Server" offering by then.

### Configuration

This are some options on how the configuration side might look like.

The original SQL Backend RFD imagined something like:

```yaml
teleport:
  storage:
    type: postgres
    addr: pgservername.postgres.database.azure.com
    database: dbname_defaulting_to_teleport
    tls:
      ca_file: path/to/azure_postgres_trust_roots.pem
    azure:
      username: pgusername@pgservername
```

Alternatively, we could provide additional aliases for the Postgres backend:

```yaml
teleport:
  storage:
    type: azurepostgres
    addr: pgservername.postgres.database.azure.com
    username: pgusername@pgservername
    database: dbname_defaulting_to_teleport
    tls:
      ca_file: path/to/azure_postgres_trust_roots.pem
```

Or we could add some "authentication type" field:

```yaml
teleport:
  storage:
    type: postgres
    addr: pgservername.postgres.database.azure.com
    username: pgusername@pgservername
    database: dbname_defaulting_to_teleport
    authentication: azure
    tls:
      ca_file: path/to/azure_postgres_trust_roots.pem
```

Other options such as checking for the `.postgres.database.azure.com` suffix in the hostname seem a bit too fragile.

Authentication will happen according to the default behavior of [the Azure Go SDK](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity), which will try to use credentials from envvars, from the IDMS accessible from Azure VMs, and from the command-line `az` tool - it would be straightforward to add some tunables to the configuration file if some specific need arose in the future.

## Audit log

The current two cloud storage options (DynamoDB on AWS and Firestore on GCP) can store both the auth backend and the events in the audit log. For user convenience, we'll implement audit log storage for the `sqlbk`/Postgres backend - an alternative would be to store the audit log in Cosmos DB, but that would require that our users set up both a Postgres server and a Cosmos DB account. As a bonus, this storage option will also be usable outside of Azure.

An `audit` table will be added to the current db schema, with indices on a `time` field and an optional `sid` field.

Assuming it's fine to store the log in the same database as the backend, a minimal configuration could be as simple as:

```yaml
teleport:
  storage:
    ...

    audit_events_uri: ['backend://']
```

With the relevant `ClusterAuditConfigSpecV2` options such as `audit_retention_period` that affect all audit logs being specified in the usual way, and future tunables for the backend-adjacent audit log being added either as extra options for the audit-aware backend or in the URI.

If we decide to go for a setup in which the audit log is completely independent of the backend storage, we could just specify a URI with all the relevant options and open a whole new connection pool for it:

```yaml
teleport:
  storage:
    ...

    audit_events_uri: ['postgres://host:123/database?sslmode=verify-full&sslrootcert=cafile&sslcert=certfile&sslkey=keyfile']
```

IAM authentication in such case would require some bespoke parameter in the query of the URI:

```yaml
teleport:
  storage:
    ...

    audit_events_uri: ['postgres://host:123/database?sslmode=verify-full&sslrootcert=cafile&user=pgusername%40pgservername&authentication=azure']
```

Additional security could be provided by only granting `INSERT` (and `SELECT`) privileges to the Postgres user on the table that stores audit events, but not `UPDATE` or `DELETE` privileges; such a setup couldn't be automatically managed by Teleport, and thus would require detailed documentation and some care.

## Session storage (wip)

Azure Blob Storage offers similar functionality to S3, including all we need for `lib/events.MultipartUploader` in the strictest sense. In addition to that, it's possible to configure containers (the equivalent of a S3 bucket) to have a time-based immutability policy that prevents moving, renaming, overwriting or deleting a blob for a fixed amount of time after it's first written, which seems to be exactly what we're currently using versioning for.

Similarly to other cloud-based session storage, it would be configured like this (details pending):

```yaml
teleport:
  storage:
    ...

    audit_sessions_uri: "azblob://accountname.blob.core.windows.net/containername"
```