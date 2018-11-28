package upload

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	get "deployer/pkg/config"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	remote_host = ""
)

func MuxShell(w io.Writer, r, e io.Reader) (chan<- string, <-chan string) {
	in := make(chan string, 3)
	out := make(chan string, 5)
	var wg sync.WaitGroup
	wg.Add(1) //for the shell itself
	go func() {
		for cmd := range in {
			wg.Add(1)
			w.Write([]byte(cmd + "\n"))
			wg.Wait()
		}
	}()

	go func() {
		var (
			buf [65 * 1024]byte
			t   int
		)
		for {
			n, err := r.Read(buf[t:])
			if err != nil {
				fmt.Println(err.Error())
				close(in)
				close(out)
				return
			}
			t += n
			result := string(buf[:t])
			if strings.Contains(result, "Username:") ||
				strings.Contains(result, "Password:") ||
				strings.Contains(result, "#") {
				out <- string(buf[:t])
				t = 0
				wg.Done()
			}
		}
	}()
	return in, out
}

func connect(user, password, host string, port int) (*sftp.Client, error) {
	var (
		auth         []ssh.AuthMethod
		addr         string
		clientConfig *ssh.ClientConfig
		sshClient    *ssh.Client
		sftpClient   *sftp.Client
		err          error
	)
	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(password))

	clientConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		Timeout:         30 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //ssh.FixedHostKey(hostKey),
	}

	// connet to ssh
	addr = fmt.Sprintf("%s:%d", host, port)
	if sshClient, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
		return nil, err
	}

	// create sftp client
	if sftpClient, err = sftp.NewClient(sshClient); err != nil {
		return nil, err
	}
	return sftpClient, nil
}
func connectSsh(user, password, host string, port int) (*ssh.Session, error) {
	var (
		auth         []ssh.AuthMethod
		addr         string
		clientConfig *ssh.ClientConfig
		client       *ssh.Client
		session      *ssh.Session
		err          error
	)
	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(password))

	clientConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		Timeout:         30 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //ssh.FixedHostKey(hostKey),
	}

	// connet to ssh
	addr = fmt.Sprintf("%s:%d", host, port)

	if client, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
		return nil, err
	}

	// create session
	if session, err = client.NewSession(); err != nil {
		return nil, err
	}

	return session, nil
}

func uploadFile(sftpClient *sftp.Client, localFilePath string, remotePath string) {
	srcFile, err := os.Open(localFilePath)
	if err != nil {
		fmt.Println("os.Open error : ", localFilePath)
		log.Fatal(err)

	}
	defer srcFile.Close()

	var remoteFileName = path.Base(localFilePath)

	dstFile, err := sftpClient.Create(path.Join(remotePath, remoteFileName))
	if err != nil {
		fmt.Println("sftpClient.Create error : ", path.Join(remotePath, remoteFileName))
		log.Fatal(err)
	}
	defer dstFile.Close()

	ff, err := ioutil.ReadAll(srcFile)
	if err != nil {
		fmt.Println("ReadAll error : ", localFilePath)
		log.Fatal(err)

	}
	dstFile.Write(ff)
	fmt.Println(localFilePath + "  copy file to \n \t\t remote(" + remote_host + ") " + remotePath + " # finished!")
}
func uploadDirectory(sftpClient *sftp.Client, localPath string, remotePath string) {
	localFiles, err := ioutil.ReadDir(localPath)
	if err != nil {
		log.Fatal("read dir list fail ", err)
	}

	for _, backupDir := range localFiles {
		localFilePath := path.Join(localPath, backupDir.Name())
		remoteFilePath := path.Join(remotePath, backupDir.Name())
		if backupDir.IsDir() {
			sftpClient.MkdirAll(remoteFilePath)
			uploadDirectory(sftpClient, localFilePath, remoteFilePath)
		} else {
			uploadFile(sftpClient, path.Join(localPath, backupDir.Name()), remotePath)
		}
	}

	fmt.Println(localPath + "  copy directory to \n \t\t remote(" + remote_host + ") " + remotePath + " # finished!")
}

func exec_shell(host string, port int, userName string, password string, commandShell string) bool {
	// log.Println(commandShell)
	session, err := connectSsh(userName, password, host, port)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()
	if strings.Contains(fmt.Sprintf("%v", commandShell), "rm ") &&
		strings.Contains(fmt.Sprintf("%v", commandShell), " -rf ") &&
		(strings.Contains(fmt.Sprintf("%v", commandShell), " / ") ||
			strings.Contains(fmt.Sprintf("%v", commandShell), " /* ") ||
			strings.Contains(fmt.Sprintf("%v", commandShell), " ./* ") ||
			strings.Contains(fmt.Sprintf("%v", commandShell), " ./ ")) {

		log.Fatal("高危操作，禁止运行！请查看配置文件参数: destination   \n\t，禁止 / 、 * 、./* 、./ 等直接影响操作系统系统目录文件!")
		return false
	}
	err = session.Run(commandShell)
	if err != nil {
		log.Fatal(err)
		return false
	}

	return true
}

/**
 * 数组去重 去空
 */
func removeDuplicatesAndEmpty(a []string) (ret []string) {
	a_len := len(a)
	for i := 0; i < a_len; i++ {
		if (i > 0 && a[i-1] == a[i]) || len(a[i]) == 0 {
			continue
		}
		ret = append(ret, a[i])
	}
	return
}

// 对外执行窗口此恢复函数
// ls  -lt ../appbak/|grep 'we_chat'|head -n 1|awk '{print $9}'
func DoRecoverT(host string, port int, userName string, password string, remotePath string, Backupdir string) {
	var (
		err        error
		sftpClient *sftp.Client
	)
	remote_host = host
	sftpClient, err = connect(userName, password, host, port)
	if err != nil {
		log.Fatal(err)
	}
	defer sftpClient.Close()
	tmpV := strings.Split(strings.Replace(remotePath, "\\", "/", -1), "/")
	tmpV2 := removeDuplicatesAndEmpty(tmpV)
	lnt := len(tmpV2) - 1
	if len(tmpV2[lnt]) == 0 {
		log.Fatal(" Error: 目录格式不正确！")
		return
	}
	_, errStat := sftpClient.Stat(Backupdir)
	if errStat != nil {
		log.Fatal(Backupdir + " " + remote_host + " remote" + " backup path not exists!")
	}

	fmt.Printf("filenameget=`ls  -lt %v/|grep '%v'|head -n 1|awk '{print $9}'`;cd %v;tar -zxvf $filenameget;rm -rf %v;scp -r %v %v;rm -rf %v; \n", Backupdir, tmpV2[lnt], Backupdir, remotePath+"/*", tmpV2[lnt]+"/*", remotePath, tmpV2[lnt])
	tmpOk2 := exec_shell(host, port, userName, password, fmt.Sprintf("filenameget=`ls  -lt %v/|grep '%v'|head -n 1|awk '{print $9}'`;cd %v;tar -zxvf $filenameget;rm -rf %v;scp -r %v %v;rm -rf %v;", Backupdir, tmpV2[lnt], Backupdir, remotePath+"/*", tmpV2[lnt]+"/*", remotePath, tmpV2[lnt]))
	if !tmpOk2 {
		log.Fatal(" 指令异常！")

	}
	log.Println("项目恢复：" + remotePath + "成功")

}

//   DoBackupT(cfg.Servers[index], cfg.Port, cfg.Username, cfg.Password, cfg.Directory, cfg.Destination, cfg.Backupdir)

// 对外执行窗口此上传文件
func DoBackupT(host string, port int, userName string, password string, Directory string, Destination string, Backupdir string) {
	var (
		err        error
		sftpClient *sftp.Client
	)
	remote_host = host
	start := time.Now()
	sftpClient, err = connect(userName, password, host, port)
	if err != nil {
		log.Fatal(err)
	}
	defer sftpClient.Close()
	_, errStat := sftpClient.Stat(Destination)
	if errStat != nil {
		log.Fatal(Destination + remote_host + " remote" + " path not exists!")
	}
	// 创建备份目录
	sftpClient.MkdirAll(Backupdir)
	_, errStat2 := sftpClient.Stat(Backupdir)
	if errStat2 != nil {
		log.Fatal(Backupdir + remote_host + " remote" + " path not exists!")
	}

	backDate := fmt.Sprintf("%v", time.Now().Format("2006-01-02"))
	// 是否需要备份处理 tmpV
	tmpV := strings.Split(strings.Replace(Directory, "\\", "/", -1), "/")
	tmpV2 := removeDuplicatesAndEmpty(tmpV)
	lnt := len(tmpV2) - 1
	backFileName := fmt.Sprintf("%v", tmpV2[lnt])
	if len(tmpV2[lnt]) == 0 {
		log.Fatal(" Error: 目录格式不正确！")
		return
	}
	if len(Backupdir) > 0 {
		log.Printf("项目开始备份：cd %v;tar -zcvf %v%v.tar.gz %v ;scp %v%v.tar.gz %v ;rm -rf %v%v.tar.gz ;\n", Destination, backFileName, backDate, backFileName, backFileName, backDate, Backupdir, backFileName, backDate)
		tmpOk1 := exec_shell(host, port, userName, password, fmt.Sprintf("cd %v;tar -zcvf %v%v.tar.gz %v ;scp %v%v.tar.gz %v ;rm -rf %v%v.tar.gz ;", Destination, backFileName, backDate, backFileName, backFileName, backDate, Backupdir, backFileName, backDate))
		if !tmpOk1 {
			log.Fatal(" 指令异常！")
		}
		log.Println("备份历史项目：" + tmpV2[lnt] + "成功")
	}
	// 删除远程项目文件
	tmpOk2 := exec_shell(host, port, userName, password, fmt.Sprintf("[[ -d '%v' ]] && rm -rf %v", Destination+"/"+tmpV2[lnt], Destination+"/"+tmpV2[lnt]))
	if !tmpOk2 {
		log.Fatal(" 指令异常！")
	}

	log.Println("项目删除：" + Destination + "/" + tmpV2[lnt] + "成功")
	_, errStat3 := sftpClient.Stat(Destination + "/" + tmpV2[lnt])
	if errStat3 != nil {
		sftpClient.MkdirAll(Destination + "/" + tmpV2[lnt])
	}
	uploadDirectory(sftpClient, Directory, Destination+"/"+tmpV2[lnt])

	elapsed := time.Since(start)

	fmt.Println("elapsed time : ", elapsed)

}

//中转机器处理函数
func DoRecover(host string, port int, userName string, password string, Directory string, Destination string, Backupdir string, config get.Config) {
	tmpV := strings.Split(strings.Replace(Directory, "\\", "/", -1), "/")
	tmpV2 := removeDuplicatesAndEmpty(tmpV)
	lnt := len(tmpV2) - 1
	if len(tmpV2[lnt]) == 0 {
		log.Fatal(" Error: 目录格式不正确！")
		return
	}
	// sshpass -pcw@123 ssh root@192.168.1.44 -o StrictHostKeyChecking=no 'aaa=`ls -lt /tmp/backupdir/`;echo -e "$aaa";'
	fmt.Printf("堡垒机 " + config.Opsip + " 上执行：" + fmt.Sprintf("sshpass -p"+password+" ssh "+userName+"@"+host+" -o StrictHostKeyChecking=no "+"\""+"if [ -d "+Backupdir+" ];then filenameget=`ls  -lt %v/|grep %v|head -n 1|awk '{print $9}'`;if [ -f $filenameget ];then cd %v;tar -zxvf $filenameget;rm -rf %v;scp -r %v %v;rm -rf %v; fi;fi;"+"\"", Backupdir, tmpV2[lnt], Backupdir, Destination+"/"+tmpV2[lnt], tmpV2[lnt], Destination, tmpV2[lnt]))
	tmpOk2 := exec_shell(config.Opsip, config.Opsport, config.Opsuser, config.Opspsswd, fmt.Sprintf("sshpass -p"+password+" ssh "+userName+"@"+host+" -o StrictHostKeyChecking=no "+"\""+"if [ -d "+Backupdir+" ];then filenameget=`ls  -lt %v/|grep %v|head -n 1|awk '{print $9}'`;if [ -f $filenameget ];then cd %v;tar -zxvf $filenameget;rm -rf %v;scp -r %v %v;rm -rf %v; fi;fi;"+"\"", Backupdir, tmpV2[lnt], Backupdir, Destination+"/"+tmpV2[lnt], tmpV2[lnt], Destination, tmpV2[lnt]))
	if !tmpOk2 {
		log.Fatal(" 指令异常！")

	}
	log.Println("项目恢复：" + Destination + "/" + tmpV2[lnt] + "成功")
}

//中转机器处理函数
func DoBackup(host string, port int, userName string, password string, Directory string, Destination string, Backupdir string, config get.Config) {
	// ftp 会话
	var (
		err        error
		sftpClient *sftp.Client
	)
	remote_host = host
	start := time.Now()
	sftpClient, err = connect(config.Opsuser, config.Opspsswd, config.Opsip, config.Opsport)
	// sftpClient, err = connect(userName, password, host, port)
	if err != nil {
		log.Fatal(err)
	}
	defer sftpClient.Close()

	// ssh 会话只能使用一次session.Run
	// ssh 会话1
	session, err := connectSsh(config.Opsuser, config.Opspsswd, config.Opsip, config.Opsport)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()
	// ssh 会话2
	session2, err := connectSsh(config.Opsuser, config.Opspsswd, config.Opsip, config.Opsport)
	if err != nil {
		log.Fatal(err)
	}
	defer session2.Close()
	// ssh 会话3
	session3, err := connectSsh(config.Opsuser, config.Opspsswd, config.Opsip, config.Opsport)
	if err != nil {
		log.Fatal(err)
	}
	defer session3.Close()

	backDate := fmt.Sprintf("%v", time.Now().Format("2006-01-02"))

	tmpV := strings.Split(strings.Replace(Directory, "\\", "/", -1), "/")
	tmpV2 := removeDuplicatesAndEmpty(tmpV)
	lnt := len(tmpV2) - 1
	if len(tmpV2[lnt]) == 0 {
		log.Fatal(" Error: 目录格式不正确！")
		return
	}
	backFileName := fmt.Sprintf("%v", tmpV2[lnt])
	_, errStat := sftpClient.Stat("/tmp/" + backFileName)
	if errStat != nil {
		sftpClient.MkdirAll("/tmp/" + backFileName)
	} else {
		err = session.Run("rm -rf /tmp/" + tmpV2[lnt] + "/*")
		log.Printf("session err:   %v\n", err)

	}
	uploadDirectory(sftpClient, Directory, "/tmp/"+tmpV2[lnt]+"/")

	// 堡垒机执行 原子服务器项目操作
	// 备份原子服务器项目
	// sshpass -pcw@123 ssh root@192.168.1.42 'scp /tmp/scppassword.log /tmp/txt/xxx/'
	// fmt.Sprintf("cd %v;tar -zcvf %v%v.tar.gz %v ;scp %v%v.tar.gz %v ;rm -rf %v%v.tar.gz ;", Destination, backFileName, backDate, backFileName, backFileName, backDate, Backupdir, backFileName, backDate)
	log.Printf("堡垒机 " + config.Opsip + " 上执行：" + fmt.Sprintf("sshpass -p"+password+" ssh "+userName+"@"+host+" -o StrictHostKeyChecking=no '"+fmt.Sprintf("if [ -d "+Destination+" ];then cd %v;tar -zcvf %v%v.tar.gz %v ;scp %v%v.tar.gz %v ;rm -rf %v%v.tar.gz ; fi;", Destination, backFileName, backDate, backFileName, backFileName, backDate, Backupdir, backFileName, backDate)+"'"))
	err = session2.Run(fmt.Sprintf("sshpass -p" + password + " ssh " + userName + "@" + host + " -o StrictHostKeyChecking=no '" + fmt.Sprintf("if [ -d "+Destination+" ];then cd %v;tar -zcvf %v%v.tar.gz %v ;scp %v%v.tar.gz %v ;rm -rf %v%v.tar.gz ; fi;", Destination, backFileName, backDate, backFileName, backFileName, backDate, Backupdir, backFileName, backDate) + "'"))
	log.Printf("session2 err:   %v\n", err)

	// 创建项目
	log.Printf("堡垒机 " + config.Opsip + " 上执行命令到目标机" + host + "：" + " scp -r " + "/tmp/" + tmpV2[lnt] + " " + userName + "@" + host + ":" + Destination)
	err = session3.Run("sshpass -p" + password + " scp -r " + "/tmp/" + tmpV2[lnt] + " " + userName + "@" + host + ":" + Destination)
	log.Printf("session3 err:   %v\n", err)
	elapsed := time.Since(start)

	fmt.Println("elapsed time : ", elapsed)
}
