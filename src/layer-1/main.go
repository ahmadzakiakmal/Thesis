package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ahmadzakiakmal/thesis/src/layer-1/app"
	"github.com/ahmadzakiakmal/thesis/src/layer-1/repository"
	"github.com/ahmadzakiakmal/thesis/src/layer-1/server"
	service_registry "github.com/ahmadzakiakmal/thesis/src/layer-1/srvreg"
	cfg "github.com/cometbft/cometbft/config"
	cmtflags "github.com/cometbft/cometbft/libs/cli/flags"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	cmtrpc "github.com/cometbft/cometbft/rpc/client/local"
	"github.com/dgraph-io/badger/v4"
	"github.com/spf13/viper"
)

var (
	homeDir      string
	httpPort     string
	postgresHost string
	isByzantine  bool
)

func init() {
	flag.StringVar(&homeDir, "cmt-home", "./node-config/simulator-node", "Path to the CometBFT config directory")
	flag.StringVar(&httpPort, "http-port", "5000", "HTTP web server port")
	flag.StringVar(&postgresHost, "postgres-host", "l1-postgres0:5432", "DB host address")
	flag.BoolVar(&isByzantine, "byzantine", false, "Byzantine Option")
}

func main() {
	// Load Config
	flag.Parse()

	if homeDir == "" {
		homeDir = os.ExpandEnv("$HOME/.cometbft")
	}
	config := cfg.DefaultConfig()
	config.SetRoot(homeDir)
	viper.SetConfigFile(fmt.Sprintf("%s/%s", homeDir, "config/config.toml"))
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Reading config: %v", err)
	}
	if err := viper.Unmarshal(config); err != nil {
		log.Fatalf("Decoding config: %v", err)
	}
	if err := config.ValidateBasic(); err != nil {
		log.Fatalf("Invalid configuration data: %v", err)
	}

	// Connect Postgresql DB
	dsn := fmt.Sprintf("postgresql://postgres:postgrespassword@%s/postgres", postgresHost)
	// Instantiate Rpc Client
	// rpcClient := cmtrpc.New(node)
	repository := repository.NewRepository()
	log.Printf("Connecting to: %s\n", dsn)
	repository.ConnectDB(dsn)

	// Initialize Badger DB
	badgerPath := filepath.Join(homeDir, "badger")
	db, err := badger.Open(badger.DefaultOptions(badgerPath))
	if err != nil {
		log.Fatalf("Opening database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatalf("Closing database: %v", err)
		}
	}()

	// Create ABCI Application
	appConfig := &app.AppConfig{
		NodeID:        filepath.Base(homeDir), // Use directory name as node ID
		RequiredVotes: 1,
		LogAllTxs:     true,
	}
	logger := cmtlog.NewTMLogger(cmtlog.NewSyncWriter(os.Stdout))

	// Initialize Service Registry
	serviceRegistry := service_registry.NewServiceRegistry(repository, logger, false)
	serviceRegistry.RegisterDefaultServices()

	app := app.NewABCIApplication(db, serviceRegistry, appConfig, logger, repository)

	// Private Validator
	pv := privval.LoadFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)

	// P2P network identity
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		log.Fatalf("failed to load node's key: %v", err)
	}

	logger, err = cmtflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel)
	if err != nil {
		log.Fatalf("failed to parse log level: %v", err)
	}

	// Initialize CometBFT node
	node, err := nm.NewNode(
		context.Background(),
		config,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(app),
		nm.DefaultGenesisDocProviderFunc(config),
		cfg.DefaultDBProvider,
		nm.DefaultMetricsProvider(config.Instrumentation),
		logger,
	)
	if err != nil {
		log.Fatalf("Creating node: %v", err)
	}

	// Pass Node ID to app
	app.SetNodeID(string(node.NodeInfo().ID()))

	// Instantiate rpc client from node
	rpcClient := cmtrpc.New(node)
	repository.SetupRpcClient(rpcClient)

	// Start CometBFT node
	node.Start()
	defer func() {
		node.Stop()
		node.Wait()
	}()

	// Start Web Server
	webserver, err := server.NewWebServer(app, httpPort, logger, node, serviceRegistry, repository)
	if err != nil {
		log.Fatalf("Creating web server: %v", err)
	}

	err = webserver.Start()
	if err != nil {
		log.Fatalf("Starting HTTP server: %v", err)
	}

	// Wait for interrupt signal to gracefully shut down the server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Create deadline to wait for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Shutdown the web server
	err = webserver.Shutdown(ctx)
	if err != nil {
		logger.Error("Shutting down HTTP web server", "err", err)
	}
	logger.Info("HTTP web server gracefully stopped")
}
