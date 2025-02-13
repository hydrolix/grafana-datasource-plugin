package testhelpers

import (
	"context"
	"log"
	"net/url"
	"path"
	"runtime"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/clickhouse"
	"github.com/testcontainers/testcontainers-go/wait"
)

type ClickhouseContainer struct {
	Container *clickhouse.ClickHouseContainer
	ConnectionString string
	Hostname string
	Username string
	Password string
	Args url.Values
	NativePort uint16
	HttpPort uint16
}

func CreateClickhouseContainer(ctx context.Context, username, password string) (*ClickhouseContainer, error) {

	_, filename, _, _ := runtime.Caller(0)
	cwd := path.Join(path.Dir(filename), "..")
	
	clickHouseContainer, err := clickhouse.Run(ctx,
		"clickhouse/clickhouse-server:latest", // TODO: set version
		clickhouse.WithUsername(username),
		clickhouse.WithPassword(password),
		// clickhouse.WithDatabase(database),
		// clickhouse.WithInitScripts(filepath.Join("testdata", "init-db.sh")),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Files: []testcontainers.ContainerFile{
					{
						ContainerFilePath: "/etc/clickhouse-server/config.d/tcconfig.xml",
						FileMode:          0644,
						HostFilePath:      path.Join(cwd, "../testdata/containers/tcconfig.xml"),
					},
				},
			},
		}),
		testcontainers.WithEnv(map[string]string{
			"CLICKHOUSE_SKIP_USER_SETUP": "1",
			// "CLICKHOUSE_ACCESS_MANAGEMENT": "1",
			// "CLICKHOUSE_USER": username,
			// "CLICKHOUSE_PASSWORD": password,
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready for connections.").
				WithOccurrence(1).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		log.Printf("failed to start CH container: %s", err)
		return nil, err
	}
	time.Sleep(5 * time.Second)

	/*
	defer func() {
		if err := testcontainers.TerminateContainer(clickHouseContainer); err != nil {
			log.Printf("failed to terminate CH container: %s", err)
		}
	}()
	*/

	connStr, err := clickHouseContainer.ConnectionString(ctx, "debug=true")
	if err != nil {
		return nil, err
	}
	httpPort, err := clickHouseContainer.MappedPort(ctx, nat.Port("8123/tcp"))
	nativePort, err := clickHouseContainer.MappedPort(ctx, nat.Port("9000/tcp"))

    u, err := url.Parse(connStr)
    if err != nil {
        panic(err)
    }

	args, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		panic(err)
	}
	
	return &ClickhouseContainer{
		Container: clickHouseContainer,
		ConnectionString:  connStr,
		Hostname: u.Hostname(),
		Username: username, Password: password,
		Args: args,
		NativePort: uint16(nativePort.Int()),
		HttpPort: uint16(httpPort.Int()),
	}, nil
}
