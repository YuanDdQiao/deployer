/*
 subprocess.call(["deployer", "-config", "settings.json"])
*/
package main

import (
	"deployer/pkg/config"
	"deployer/pkg/upload"
	"flag"
	"fmt"
	"log"
	"runtime"
	"strconv"
)

type Value interface {
	String() string
	Set(string) error
}
type arrayFlags []string

var sshAddrs arrayFlags
var configFile = flag.String("config", "", "deployer config file")
var sshPort = flag.String("port", "", "remote ssh port")
var sshUser = flag.String("user", "", "remote ssh user")
var sshPass = flag.String("password", "", "remote ssh password")
var srcDir = flag.String("src", "", "local dir")
var dstDir = flag.String("dst", "", "destination dir")
var bakDir = flag.String("bak", "", "backup to")

// Value ...
func (i *arrayFlags) String() string {
	return fmt.Sprint(*i)
}

// Set 方法是flag.Value接口, 设置flag Value的方法.
// 通过多个flag指定的值， 所以我们追加到最终的数组上.
func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	// runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GOMAXPROCS(2)
	flag.Var(&sshAddrs, "host", "remote ssh addr")

	flag.Parse()

	cfg, err := deploy.LoadConfig(*configFile)
	if err != nil {
		fmt.Errorf("Error: load configfile %v ..", err)
		return
	}

	if len(sshAddrs) > 0 {
		cfg.Servers = sshAddrs
	}

	if len(*sshPort) > 0 {
		cfg.Port, err = strconv.Atoi(*sshPort)
		if err != nil {
			fmt.Errorf("Error: ssh port %v is not integer ..", err)
			return
		}
	}
	if len(*sshUser) > 0 {
		cfg.Username = *sshUser
	}

	if len(*sshPass) > 0 {
		cfg.Password = *sshPass
	}

	if len(*srcDir) > 0 {
		cfg.Directory = *srcDir
	}

	if len(*dstDir) > 0 {
		cfg.Destination = *dstDir
	}
	if len(*bakDir) > 0 {
		cfg.Backupdir = *bakDir
	}
	if len(cfg.Backupdir) < 0 {
		log.Printf("项目不需要备份...\n")
	} else {
		log.Printf("旧项目备份地址: %v...\n", cfg.Backupdir)
	}

	if len(cfg.Servers) > 1 {
		for i, ipaddr := range cfg.Servers {
			// fmt.Printf("查看IP 是否正确：%v", string(ipaddr[i]))
			upload.DoBackup(string(ipaddr[i]), cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir)
		}
	} else {
		// fmt.Printf("查看IP 是否正确：%v", cfg.Servers)
		upload.DoBackup(cfg.Servers[0], cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir)
	}
}
