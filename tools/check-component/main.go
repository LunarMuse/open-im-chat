// Copyright © 2023 OpenIM. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/openimsdk/chat/pkg/common/config"
	"github.com/openimsdk/chat/pkg/common/imapi"
	"github.com/openimsdk/tools/db/mongoutil"
	"github.com/openimsdk/tools/db/redisutil"
	"github.com/openimsdk/tools/discovery/etcd"
	"github.com/openimsdk/tools/mcontext"
	"github.com/openimsdk/tools/system/program"
	"github.com/openimsdk/tools/utils/idutil"
	"github.com/openimsdk/tools/utils/runtimeenv"
)

const maxRetry = 180

func CheckEtcd(ctx context.Context, config *config.Etcd) error {
	return etcd.Check(ctx, config.Address, "/check_chat_component",
		true,
		etcd.WithDialTimeout(10*time.Second),
		etcd.WithMaxCallSendMsgSize(20*1024*1024),
		etcd.WithUsernameAndPassword(config.Username, config.Password))
}

func CheckMongo(ctx context.Context, config *config.Mongo) error {
	return mongoutil.Check(ctx, config.Build())
}

func CheckRedis(ctx context.Context, config *config.Redis) error {
	return redisutil.Check(ctx, config.Build())
}

func CheckOpenIM(ctx context.Context, apiURL, secret, adminUserID string, redisConf *config.Redis, interval int) error {
	imAPI := imapi.New(apiURL, secret, adminUserID)
	_, err := imAPI.GetAdminTokenServer(mcontext.SetOperationID(ctx, "CheckOpenIM"+idutil.OperationIDGenerator()), adminUserID)
	return err
}

func initConfig(configDir string) (*config.Mongo, *config.Redis, *config.Discovery, *config.Share, error) {
	var (
		mongoConfig     = &config.Mongo{}
		redisConfig     = &config.Redis{}
		discoveryConfig = &config.Discovery{}
		shareConfig     = &config.Share{}
	)

	runtimeEnv := runtimeenv.PrintRuntimeEnvironment()

	err := config.Load(configDir, config.MongodbConfigFileName, config.EnvPrefixMap[config.MongodbConfigFileName], runtimeEnv, mongoConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	err = config.Load(configDir, config.RedisConfigFileName, config.EnvPrefixMap[config.RedisConfigFileName], runtimeEnv, redisConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	err = config.Load(configDir, config.DiscoveryConfigFileName, config.EnvPrefixMap[config.DiscoveryConfigFileName], runtimeEnv, discoveryConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	err = config.Load(configDir, config.ShareFileName, config.EnvPrefixMap[config.ShareFileName], runtimeEnv, shareConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return mongoConfig, redisConfig, discoveryConfig, shareConfig, nil
}

func main() {
	var index int
	var configDir string
	flag.IntVar(&index, "i", 0, "Index number")
	defaultConfigDir := filepath.Join("..", "..", "..", "..", "..", "config")
	flag.StringVar(&configDir, "c", defaultConfigDir, "Configuration dir")
	flag.Parse()

	fmt.Printf("Index: %d, Config Path: %s\n", index, configDir)

	mongoConfig, redisConfig, zookeeperConfig, shareConfig, err := initConfig(configDir)
	if err != nil {
		program.ExitWithError(err)
	}

	ctx := context.Background()
	err = performChecks(ctx, mongoConfig, redisConfig, zookeeperConfig, shareConfig, maxRetry)
	if err != nil {
		// Assume program.ExitWithError logs the error and exits.
		// Replace with your error handling logic as necessary.
		program.ExitWithError(err)
	}
}

func performChecks(ctx context.Context, mongoConfig *config.Mongo, redisConfig *config.Redis, discovery *config.Discovery, shareConfig *config.Share, maxRetry int) error {
	checksDone := make(map[string]bool)

	checks := map[string]func(ctx context.Context) error{
		"Mongo": func(ctx context.Context) error {
			return CheckMongo(ctx, mongoConfig)
		},
		"Redis": func(ctx context.Context) error {
			return CheckRedis(ctx, redisConfig)
		},
		"OpenIM": func(ctx context.Context) error {
			return CheckOpenIM(ctx, shareConfig.OpenIM.ApiURL, shareConfig.OpenIM.Secret, shareConfig.OpenIM.AdminUserID, redisConfig, shareConfig.OpenIM.TokenRefreshInterval)
		},
	}

	if discovery.Enable == "etcd" {
		checks["Etcd"] = func(ctx context.Context) error {
			return CheckEtcd(ctx, &discovery.Etcd)
		}
	}

	for i := 0; i < maxRetry; i++ {
		allSuccess := true
		for name, check := range checks {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if !checksDone[name] {
				if err := check(ctx); err != nil {
					fmt.Printf("%s check failed: %v\n", name, err)
					allSuccess = false
				} else {
					fmt.Printf("%s check succeeded.\n", name)
					checksDone[name] = true
				}
			}
			cancel()
		}

		if allSuccess {
			fmt.Println("All components checks passed successfully.")
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("not all components checks passed successfully after %d attempts", maxRetry)
}
