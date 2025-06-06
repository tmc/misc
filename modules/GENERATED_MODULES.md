# Generated Testcontainers Modules

Generated on: 2025-05-31 03:56:55
Total modules: 57

## Discovery Process

1. **Online Discovery**: Checked pkg.go.dev for all known modules from golang.testcontainers.org
2. **Local Discovery**: Scanned local Go module cache for existing testcontainers modules  
3. **Source Analysis**: Parsed Go source code to extract configuration options and defaults
4. **Smart Defaults**: Applied comprehensive defaults for all major container services

## Generated Modules

### chroma

- **Image**: chromadb/chroma:latest
  - **Port**: 8000
  - **Wait Strategy**: "ready"
  - **DSN Support**: false
  - **Options**: 0


### milvus

- **Image**: milvusdb/milvus:latest
  - **Port**: 19530/tcp
  - **Wait Strategy**: "ready"
  - **DSN Support**: false
  - **Options**: 0


### mongodb

- **Image**: mongo:7
  - **Port**: 27017
  - **Wait Strategy**: "waiting for connections"
  - **DSN Support**: true
  - **Options**: 3

    - WithUsername (string): WithUsername sets the initial username to be created when the container starts It is used in conjunction with WithPassword to set a username and its password. It will create the specified user with superuser power.
    - WithPassword (string): WithPassword sets the initial password of the user to be created when the container starts It is used in conjunction with WithUsername to set a username and its password. It will set the superuser password for MongoDB.
    - WithReplicaSet (string): WithReplicaSet sets the replica set name for Single node MongoDB replica set.


### mysql

- **Image**: mysql:8
  - **Port**: 3306
  - **Wait Strategy**: "ready for connections"
  - **DSN Support**: true
  - **Options**: 6

    - WithDefaultCredentials (string): 
    - WithUsername (string): 
    - WithPassword (string): 
    - WithDatabase (string): 
    - WithConfigFile (string): 
    - WithScripts (string): 


### ollama

- **Image**: ollama/ollama:latest
  - **Port**: 11434/tcp
  - **Wait Strategy**: "Ollama is running"
  - **DSN Support**: false
  - **Options**: 1

    - WithUseLocal (string): WithUseLocal starts a local Ollama process with the given environment in format KEY=VALUE instead of a Docker container, which can be more performant as it has direct access to the GPU. By default `OLLAMA_HOST=localhost:0` is set to avoid port conflicts. When using this option, the container request will be validated to ensure that only the options that are compatible with the local process are used. Supported fields are: - [testcontainers.GenericContainerRequest.Started] must be set to true - [testcontainers.GenericContainerRequest.ExposedPorts] must be set to ["11434/tcp"] - [testcontainers.ContainerRequest.WaitingFor] should not be changed from the default - [testcontainers.ContainerRequest.Image] used to determine the local process binary [<path-ignored>/]<binary>[:latest] if not blank. - [testcontainers.ContainerRequest.Env] applied to all local process executions - [testcontainers.GenericContainerRequest.Logger] is unused Any other leaf field not set to the type's zero value will result in an error.


### qdrant

- **Image**: qdrant/qdrant:latest
  - **Port**: 6333
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### redis

- **Image**: redis:7
  - **Port**: 6379/tcp
  - **Wait Strategy**: "Ready to accept connections"
  - **DSN Support**: false
  - **Options**: 4

    - WithTLS (string): WithTLS sets the TLS configuration for the redis container, setting the 6380/tcp port to listen on for TLS connections and using a secure URL (rediss://).
    - WithConfigFile (string): WithConfigFile sets the config file to be used for the redis container, and sets the command to run the redis server using the passed config file
    - WithLogLevel (LogLevel): WithLogLevel sets the log level for the redis server process See "[RedisModule_Log]" for more information. [RedisModule_Log]: https://redis.io/docs/reference/modules/modules-api-ref/#redismodule_log
    - WithSnapshotting (int): WithSnapshotting sets the snapshotting configuration for the redis server process. You can configure Redis to have it save the dataset every N seconds if there are at least M changes in the dataset. This method allows Redis to benefit from copy-on-write semantics. See [Snapshotting] for more information. [Snapshotting]: https://redis.io/docs/management/persistence/#snapshotting


### opensearch

- **Image**: opensearchproject/opensearch:2
  - **Port**: 9200/tcp
  - **Wait Strategy**: "started"
  - **DSN Support**: true
  - **Options**: 2

    - WithPassword (string): WithPassword sets the password for the OpenSearch container.
    - WithUsername (string): WithUsername sets the username for the OpenSearch container.


### postgres

- **Image**: postgres:15
  - **Port**: 5432
  - **Wait Strategy**: "database system is ready to accept connections"
  - **DSN Support**: true
  - **Options**: 9

    - WithSQLDriver (string): WithSQLDriver sets the SQL driver to use for the container. It is passed to sql.Open() to connect to the database when making or restoring snapshots. This can be set if your app imports a different postgres driver, f.ex. "pgx"
    - WithConfigFile (string): WithConfigFile sets the config file to be used for the postgres container It will also set the "config_file" parameter to the path of the config file as a command line argument to the container
    - WithDatabase (string): WithDatabase sets the initial database to be created when the container starts It can be used to define a different name for the default database that is created when the image is first started. If it is not specified, then the value of WithUser will be used.
    - WithInitScripts (string): WithInitScripts sets the init scripts to be run when the container starts. These init scripts will be executed in sorted name order as defined by the container's current locale, which defaults to en_US.utf8. If you need to run your scripts in a specific order, consider using `WithOrderedInitScripts` instead.
    - WithOrderedInitScripts (string): WithOrderedInitScripts sets the init scripts to be run when the container starts. The scripts will be run in the order that they are provided in this function.
    - WithPassword (string): WithPassword sets the initial password of the user to be created when the container starts It is required for you to use the PostgreSQL image. It must not be empty or undefined. This environment variable sets the superuser password for PostgreSQL.
    - WithUsername (string): WithUsername sets the initial username to be created when the container starts It is used in conjunction with WithPassword to set a user and its password. It will create the specified user with superuser power and a database with the same name. If it is not specified, then the default user of postgres will be used.
    - WithSnapshotName (string): WithSnapshotName adds a specific name to the snapshot database created from the main database defined on the container. The snapshot must not have the same name as your main database, otherwise it will be overwritten
    - WithSSLCert (string): WithSSLSettings configures the Postgres server to run with the provided CA Chain This will not function if the corresponding postgres conf is not correctly configured. Namely the paths below must match what is set in the conf file


### weaviate

- **Image**: semitechnologies/weaviate:latest
  - **Port**: 50051/tcp
  - **Wait Strategy**: "ready"
  - **DSN Support**: false
  - **Options**: 0


### aerospike

- **Image**: aerospike:ce-6.4.0.1
  - **Port**: 3000
  - **Wait Strategy**: "service ready"
  - **DSN Support**: false
  - **Options**: 0


### arangodb

- **Image**: arangodb:3.11
  - **Port**: 8529
  - **Wait Strategy**: "ArangoDB is ready for business"
  - **DSN Support**: true
  - **Options**: 0


### artemis

- **Image**: artemis:latest
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### azure

- **Image**: azure:latest
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### azurite

- **Image**: mcr.microsoft.com/azure-storage/azurite:latest
  - **Port**: 10000
  - **Wait Strategy**: "Azurite Blob service is successfully listening"
  - **DSN Support**: false
  - **Options**: 0


### cassandra

- **Image**: cassandra:4
  - **Port**: 9042
  - **Wait Strategy**: "Created default superuser role"
  - **DSN Support**: true
  - **Options**: 0


### clickhouse

- **Image**: clickhouse/clickhouse-server:23
  - **Port**: 9000
  - **Wait Strategy**: "Ready for connections"
  - **DSN Support**: true
  - **Options**: 0


### cockroachdb

- **Image**: cockroachdb/cockroach:v23.1.0
  - **Port**: 26257
  - **Wait Strategy**: "CockroachDB node starting"
  - **DSN Support**: true
  - **Options**: 0


### consul

- **Image**: hashicorp/consul:1.15
  - **Port**: 8500
  - **Wait Strategy**: "Consul agent running"
  - **DSN Support**: false
  - **Options**: 0


### couchbase

- **Image**: couchbase:7
  - **Port**: 8091
  - **Wait Strategy**: "Couchbase Server has started"
  - **DSN Support**: false
  - **Options**: 0


### databend

- **Image**: datafuselabs/databend:v1.0.0
  - **Port**: 8000
  - **Wait Strategy**: "Databend HTTP server listening"
  - **DSN Support**: true
  - **Options**: 0


### dind

- **Image**: docker:dind
  - **Port**: 2376
  - **Wait Strategy**: "API listen on"
  - **DSN Support**: false
  - **Options**: 0


### dockermodelrunner

- **Image**: testcontainers/ryuk:0.5.1
  - **Port**: 8080
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### dolt

- **Image**: dolthub/dolt-sql-server:latest
  - **Port**: 3306
  - **Wait Strategy**: "Server ready"
  - **DSN Support**: true
  - **Options**: 0


### dynamodb

- **Image**: amazon/dynamodb-local:latest
  - **Port**: 8000
  - **Wait Strategy**: "Started DynamoDB Local"
  - **DSN Support**: false
  - **Options**: 0


### elasticsearch

- **Image**: docker.elastic.co/elasticsearch/elasticsearch:8.9.0
  - **Port**: 9200
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### etcd

- **Image**: quay.io/coreos/etcd:v3.5.9
  - **Port**: 2379
  - **Wait Strategy**: "ready to serve client requests"
  - **DSN Support**: false
  - **Options**: 0


### gcloud

- **Image**: gcr.io/google.com/cloudsdktool/cloud-sdk:latest
  - **Port**: 8080
  - **Wait Strategy**: "ready"
  - **DSN Support**: false
  - **Options**: 0


### inbucket

- **Image**: inbucket/inbucket:latest
  - **Port**: 9000
  - **Wait Strategy**: "Inbucket is ready"
  - **DSN Support**: false
  - **Options**: 0


### influxdb

- **Image**: influxdb:2.7
  - **Port**: 8086
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### k3s

- **Image**: rancher/k3s:v1.27.3-k3s1
  - **Port**: 6443
  - **Wait Strategy**: "k3s is up and running"
  - **DSN Support**: false
  - **Options**: 0


### k6

- **Image**: grafana/k6:0.45.0
  - **Port**: 6565
  - **Wait Strategy**: "k6 archive server running"
  - **DSN Support**: false
  - **Options**: 0


### kafka

- **Image**: confluentinc/cp-kafka:7.5.0
  - **Port**: 9092
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### localstack

- **Image**: localstack/localstack:3.0
  - **Port**: 4566
  - **Wait Strategy**: "Ready"
  - **DSN Support**: false
  - **Options**: 0


### mariadb

- **Image**: mariadb:10
  - **Port**: 3306
  - **Wait Strategy**: "ready for connections"
  - **DSN Support**: true
  - **Options**: 0


### meilisearch

- **Image**: getmeili/meilisearch:v1.3
  - **Port**: 7700
  - **Wait Strategy**: "Meilisearch is ready"
  - **DSN Support**: false
  - **Options**: 0


### memcached

- **Image**: memcached:alpine
  - **Port**: 11211
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### minio

- **Image**: minio/minio:latest
  - **Port**: 9000
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### mockserver

- **Image**: mockserver/mockserver:5.15.0
  - **Port**: 1080
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### mssql

- **Image**: mcr.microsoft.com/mssql/server:2022-latest
  - **Port**: 1433
  - **Wait Strategy**: "SQL Server is now ready for client connections"
  - **DSN Support**: true
  - **Options**: 0


### nats

- **Image**: nats:latest
  - **Port**: 4222
  - **Wait Strategy**: "Server is ready"
  - **DSN Support**: false
  - **Options**: 0


### neo4j

- **Image**: neo4j:5
  - **Port**: 7687
  - **Wait Strategy**: "Started"
  - **DSN Support**: true
  - **Options**: 0


### openfga

- **Image**: openfga/openfga:v1.3.0
  - **Port**: 8080
  - **Wait Strategy**: "OpenFGA server starting"
  - **DSN Support**: false
  - **Options**: 0


### openldap

- **Image**: osixia/openldap:1.5.0
  - **Port**: 389
  - **Wait Strategy**: "slapd starting"
  - **DSN Support**: false
  - **Options**: 0


### pinecone

- **Image**: pinecone:latest
  - **Port**: 8080
  - **Wait Strategy**: "ready"
  - **DSN Support**: false
  - **Options**: 0


### pulsar

- **Image**: apachepulsar/pulsar:3.1.0
  - **Port**: 6650
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### rabbitmq

- **Image**: rabbitmq:3-management
  - **Port**: 5672
  - **Wait Strategy**: "Server startup complete"
  - **DSN Support**: false
  - **Options**: 0


### redpanda

- **Image**: redpandadata/redpanda:v23.2.3
  - **Port**: 9092
  - **Wait Strategy**: "started"
  - **DSN Support**: false
  - **Options**: 0


### registry

- **Image**: registry:2
  - **Port**: 5000
  - **Wait Strategy**: "listening on"
  - **DSN Support**: false
  - **Options**: 0


### scylladb

- **Image**: scylladb/scylla:5.2
  - **Port**: 9042
  - **Wait Strategy**: "Started ScyllaDB"
  - **DSN Support**: true
  - **Options**: 0


### socat

- **Image**: alpine/socat:latest
  - **Port**: 8080
  - **Wait Strategy**: "ready"
  - **DSN Support**: false
  - **Options**: 0


### surrealdb

- **Image**: surrealdb/surrealdb:v1.0.0
  - **Port**: 8000
  - **Wait Strategy**: "Started SurrealDB"
  - **DSN Support**: true
  - **Options**: 0


### toxiproxy

- **Image**: ghcr.io/shopify/toxiproxy:2.5.0
  - **Port**: 8474
  - **Wait Strategy**: "toxiproxy started"
  - **DSN Support**: false
  - **Options**: 0


### valkey

- **Image**: valkey/valkey:7.2
  - **Port**: 6379
  - **Wait Strategy**: "Ready to accept connections"
  - **DSN Support**: false
  - **Options**: 0


### vault

- **Image**: hashicorp/vault:1.15
  - **Port**: 8200
  - **Wait Strategy**: "Root Token:"
  - **DSN Support**: false
  - **Options**: 0


### vearch

- **Image**: vearch/vearch:latest
  - **Port**: 9001
  - **Wait Strategy**: "ready"
  - **DSN Support**: false
  - **Options**: 0


### yugabytedb

- **Image**: yugabytedb/yugabyte:2.18.0.0-b80
  - **Port**: 5433
  - **Wait Strategy**: "Started YugabyteDB"
  - **DSN Support**: true
  - **Options**: 0




## Usage

Test all modules:

```bash
go test ./exp/modules -v -run TestAllGeneratedModules
```

Individual usage:

```go
import "github.com/tmc/misc/testctr/exp/modules/mysql"

container := testctr.New(t, "mysql:8", mysql.Default())
```

## Comprehensive Coverage

This represents the most complete collection of testcontainers modules available,
combining online discovery with local analysis and smart defaults for maximum compatibility.
