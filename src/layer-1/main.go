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
	"github.com/ahmadzakiakmal/thesis/src/layer-1/server"
	"github.com/ahmadzakiakmal/thesis/src/layer-1/server/models"
	service_registry "github.com/ahmadzakiakmal/thesis/src/layer-1/service-registry"
	cfg "github.com/cometbft/cometbft/config"
	cmtflags "github.com/cometbft/cometbft/libs/cli/flags"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	"github.com/dgraph-io/badger/v4"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	homeDir      string
	httpPort     string
	postgresHost string
	DB           *gorm.DB
)

func init() {
	flag.StringVar(&homeDir, "cmt-home", "", "Path to the CometBFT config directory")
	flag.StringVar(&httpPort, "http-port", "5000", "HTTP web server port")
	flag.StringVar(&postgresHost, "postgres-host", "postgres-node0:5432", "DB address")
}

func main() {
	//? Load Config
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

	//? Connect Postgresql DB
	ConnectDB()

	//? Initialize Badger DB
	dbPath := filepath.Join(homeDir, "badger")
	db, err := badger.Open(badger.DefaultOptions(dbPath))
	if err != nil {
		log.Fatalf("Opening database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatalf("Closing database: %v", err)
		}
	}()

	//? Initialize Service Registry
	serviceRegistry := service_registry.NewServiceRegistry(DB)
	serviceRegistry.RegisterDefaultServices()

	//? Create DeWS Application
	dewsConfig := &app.DeWSConfig{
		NodeID:        filepath.Base(homeDir), // Use directory name as node ID
		RequiredVotes: 2,                      // For demo, 2 votes required
		LogAllTxs:     true,
	}
	logger := cmtlog.NewTMLogger(cmtlog.NewSyncWriter(os.Stdout))

	app := app.NewDeWSApplication(db, serviceRegistry, dewsConfig, logger)

	//? Private Validator
	pv := privval.LoadFilePV(
		config.PrivValidatorKeyFile(),
		config.PrivValidatorStateFile(),
	)

	//? P2P network identity
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		log.Fatalf("failed to load node's key: %v", err)
	}

	logger, err = cmtflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel)
	if err != nil {
		log.Fatalf("failed to parse log level: %v", err)
	}

	//? Initialize CometBFT node
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

	//? Pass Node ID to app
	app.SetNodeID(string(node.NodeInfo().ID()))

	//? Start CometBFT node
	node.Start()
	defer func() {
		node.Stop()
		node.Wait()
	}()

	//? Start DeWS Web Server
	webserver, err := server.NewDeWSWebServer(app, httpPort, logger, node, serviceRegistry, DB)
	if err != nil {
		log.Fatalf("Creating web server: %v", err)
	}

	err = webserver.Start()
	if err != nil {
		log.Fatalf("Starting HTTP server: %v", err)
	}

	//? Wait for interrupt signal to gracefully shut down the server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	//? Create deadline to wait for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	//? Shutdown the web server
	err = webserver.Shutdown(ctx)
	if err != nil {
		logger.Error("Shutting down HTTP web server", "err", err)
	}
	logger.Info("HTTP web server gracefully stopped")
}

func ConnectDB() {
	var err error
	dsn := fmt.Sprintf("postgresql://postgres:postgrespassword@%s/dewsdb", postgresHost)
	log.Printf("Connecting to: %s\n", dsn)

	for i := 0; i < 5; i++ {
		log.Printf("Connection attempt %d...\n", i+1)
		DB, err = gorm.Open(postgres.Open(dsn))
		if err != nil {
			log.Printf("Connection attempt %d, failed: %v\n", i+1, err)
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}

	if err != nil {
		log.Fatal("Connection to db failed: ", err.Error())
	}
	DB.AutoMigrate(&models.User{})

	log.Print("Connected to DB")
}
