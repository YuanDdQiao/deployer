package upload

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	remote_host = ""
)

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
	session, err := connectSsh(userName, password, host, port)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()
	if strings.Contains(fmt.Sprintf("%v", "rm  -rf "), "rm ") &&
		strings.Contains(fmt.Sprintf("%v", "rm  -rf "), " -rf ") &&
		(strings.Contains(fmt.Sprintf("%v", "rm  -rf "), " / ") ||
			strings.Contains(fmt.Sprintf("%v", "rm  -rf "), " /* ") ||
			strings.Contains(fmt.Sprintf("%v", "rm  -rf "), " ./* ") ||
			strings.Contains(fmt.Sprintf("%v", "rm  -rf "), " ./ ")) {

		log.Fatal("高危操作，禁止运行！请查看配置文件参数: destination   \n\t，禁止 / 、 * 、./* 、./ 等直接影响操作系统系统目录文件!")
		return false
	}
	session.Run(commandShell)
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

// 对外执行窗口
func DoBackup(host string, port int, userName string, password string, localPath string, remotePath string, Backupdir string) {
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
	sftpClient.MkdirAll(remotePath)
	_, errStat := sftpClient.Stat(remotePath)
	if errStat != nil {
		log.Fatal(remotePath + remote_host + " remote" + " path not exists!")
	}
	// 创建备份目录
	sftpClient.MkdirAll(Backupdir)
	_, errStat2 := sftpClient.Stat(Backupdir)
	if errStat2 != nil {
		log.Fatal(Backupdir + remote_host + " remote" + " path not exists!")
	}

	// 是否需要备份处理 tmpV
	if len(Backupdir) > 0 {
		tmpV := strings.Split(strings.Replace(remotePath, "\\", "/", -1), "/")
		tmpV2 := removeDuplicatesAndEmpty(tmpV)
		lnt := len(tmpV2) - 1
		if len(tmpV2[lnt]) == 0 {
			log.Fatal(" Error: 目录格式不争取！")
			return
		}
		log.Printf("项目开始备份：tar -zcvf %v%v%v.tar %v \n", Backupdir, fmt.Sprintf("%v", tmpV2[lnt]), fmt.Sprintf("%v", time.Now().Format("2006-01-02")), remotePath)
		tmpOk1 := exec_shell(host, port, userName, password, fmt.Sprintf("tar -zcvf %v%v%v.tar %v", Backupdir, fmt.Sprintf("%v", tmpV2[lnt]), fmt.Sprintf("%v", time.Now().Format("2006-01-02")), remotePath))
		if !tmpOk1 {
			log.Fatal(" 指令异常！")
		}
		log.Println("备份历史项目：" + remotePath + "成功")
	}
	// 删除远程项目文件
	tmpOk2 := exec_shell(host, port, userName, password, fmt.Sprintf("[[ -d '%v' ]] && rm -rf %v", remotePath, remotePath))
	if !tmpOk2 {
		log.Fatal(" 指令异常！")
	}
	log.Println("项目删除：" + remotePath + "成功")

	uploadDirectory(sftpClient, localPath, remotePath)

	elapsed := time.Since(start)

	fmt.Println("elapsed time : ", elapsed)

}
