package deploy

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"regexp"
)

const configFileSizeLimit = 10 << 20

var defaultConfig = &struct {
	netTimeout   int64
	fileDeadtime string
}{
	netTimeout:   15,
	fileDeadtime: "24h",
}

//有了`json:network`这种注释，后面json解析就可以把相应的数据塞到对应的结构里面来
type Config struct {
	Servers     []string `json:"servers"`
	Port        int      `json:"port"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	Project     string   `json:"project"`
	Directory   string   `json:"directory"`
	Destination string   `json:"destination"`
}

func LoadConfig(name string) (config Config, err error) {
	config_file, err := os.Open(name)
	if err != nil {
		emit("Failed to open config file '%s': %s\n", name, err)
		return
	}

	fi, _ := config_file.Stat()
	if size := fi.Size(); size > (configFileSizeLimit) {
		emit("config file (%q) size exceeds reasonable limit (%d) - aborting", name, size)
		return // REVU: shouldn't this return an error, then?
	}

	if fi.Size() == 0 {
		emit("config file (%q) is empty, skipping", name)
		return
	}

	buffer := make([]byte, fi.Size())
	_, err = config_file.Read(buffer)
	emit("\n %s\n", buffer)

	buffer, err = StripComments(buffer) //去掉注释
	if err != nil {
		emit("Failed to strip comments from json: %s\n", err)
		return
	}

	buffer = []byte(os.ExpandEnv(string(buffer))) //特殊,处理环境变量

	err = json.Unmarshal(buffer, &config) //解析json格式数据
	if err != nil {
		emit("Failed unmarshalling json: %s\n", err)
		return
	}
	return config, nil
}

func StripComments(data []byte) ([]byte, error) {
	data = bytes.Replace(data, []byte("\r"), []byte(""), 0) // Windows
	lines := bytes.Split(data, []byte("\n"))                //split to muli lines
	filtered := make([][]byte, 0)

	for _, line := range lines {
		match, err := regexp.Match(`^\s*#`, line)
		if err != nil {
			return nil, err
		}
		if !match {
			filtered = append(filtered, line)
		}
	}

	return bytes.Join(filtered, []byte("\n")), nil
}

func emit(msgfmt string, args ...interface{}) {
	log.Printf(msgfmt, args...)
}
