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

// 安装 yum install sshpass -y
type arrayFlags []string

var sshAddrs arrayFlags
var configFile = flag.String("config", "", "deployer config file")
var sshPort = flag.String("port", "", "remote ssh port")
var sshUser = flag.String("user", "", "remote ssh user")
var sshPass = flag.String("password", "", "remote ssh password")
var srcDir = flag.String("src", "", "local dir")
var dstDir = flag.String("dst", "", "destination dir")
var bakDir = flag.String("bak", "", "backup to")
var recoverDir = flag.String("recover", "false", "recover to")

// 堡垒机配置
// Opsip    string `json:"opsip"`
// Opsuser  int    `json:"opsuser"`
// Opsport  string `json:"opsport"`
// Opspsswd string `json:"opspsswd"`
var OpssshAddr = flag.String("opsip", "", "remote ssh addr")
var OpssshPort = flag.String("opsport", "", "remote ssh port")
var OpssshUser = flag.String("opsuser", "", "remote ssh user")
var OpssshPass = flag.String("opspsswd", "", "remote ssh password")

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
	// 堡垒机配置
	if len(*OpssshAddr) > 0 {
		cfg.Opsip = *OpssshAddr
	}

	if len(*OpssshPort) > 0 {
		cfg.Opsport, err = strconv.Atoi(*OpssshPort)
		if err != nil {
			fmt.Errorf("Error: Ops ssh port %v is not integer ..", err)
			return
		}
	}
	if len(*OpssshUser) > 0 {
		cfg.Opsuser = *OpssshUser
	}

	if len(*OpssshPass) > 0 {
		cfg.Opspsswd = *OpssshPass
	}
	// 堡垒机 是否 做中转机器
	if len(cfg.Opsip) > 0 || cfg.Opsport > 0 || len(cfg.Opsuser) > 0 || len(cfg.Opspsswd) > 0 {
		// 是否恢复上一次操作
		if *recoverDir == "true" {
			fmt.Printf("不做部署操作，只做恢复上一次的备份：%v \n", *recoverDir)
			if len(cfg.Servers) > 1 {
				for index := 0; index < len(cfg.Servers); index++ {
					upload.DoRecover(cfg.Servers[index], cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir, cfg)
				}
			} else {
				upload.DoRecover(cfg.Servers[0], cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir, cfg)
			}
			return
		}

		if len(cfg.Servers) > 1 {
			for index := 0; index < len(cfg.Servers); index++ {
				// fmt.Printf("查看IP 是否正确：%v", cfg.Servers[index])
				upload.DoBackup(cfg.Servers[index], cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir, cfg)
			}
		} else {
			// fmt.Printf("查看IP 是否正确：%v", cfg.Servers)
			upload.DoBackup(cfg.Servers[0], cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir, cfg)
		}

	} else {
		// 是否恢复上一次操作
		if *recoverDir == "true" {
			fmt.Printf("不做部署操作，只做恢复上一次的备份：%v \n", *recoverDir)
			if len(cfg.Servers) > 1 {
				for index := 0; index < len(cfg.Servers); index++ {
					upload.DoRecoverT(cfg.Servers[index], cfg.Port, cfg.Username, cfg.Password, cfg.Destination, cfg.Backupdir)
				}
			} else {
				upload.DoRecoverT(cfg.Servers[0], cfg.Port, cfg.Username, cfg.Password, cfg.Destination, cfg.Backupdir)
			}
			return
		}

		if len(cfg.Servers) > 1 {
			for index := 0; index < len(cfg.Servers); index++ {
				// fmt.Printf("查看IP 是否正确：%v", cfg.Servers[index])
				upload.DoBackupT(cfg.Servers[index], cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir)
			}
		} else {
			// fmt.Printf("查看IP 是否正确：%v", cfg.Servers)
			upload.DoBackupT(cfg.Servers[0], cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir)
		}

	}
}
