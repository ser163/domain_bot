package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Domains      []string `yaml:"domains"`
	Days         int      `yaml:"days"`
	External     string   `yaml:"external"`
	Method       string   `yaml:"method"`
	ArgsTemplate string   `yaml:"args"`
}

func main() {
	configFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}

	var config Config
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		panic(err)
	}

	for _, domain := range config.Domains {
		expirationDate, err := queryExpirationDate(domain)
		if err != nil {
			fmt.Printf("查询域名 %s 到期时间失败：%v\n", domain, err)
			continue
		}

		daysLeft := int(expirationDate.Sub(time.Now()).Hours() / 24)

		fmt.Printf("域名 %s 将在 %s 过期，还剩 %d 天\n", domain, expirationDate.Format("2006-01-02"), daysLeft)

		if daysLeft < config.Days {
			message := fmt.Sprintf("域名 %s 将在 %s 过期，还剩 %d 天", domain, expirationDate.Format("2006-01-02"), daysLeft)
			err = runExternalProgram(config.External, message, config.Method, config.ArgsTemplate)
			if err != nil {
				fmt.Printf("运行外部程序 %s 出错: %v\n", config.External, err)
			}
		}
	}
}

func queryExpirationDate(domain string) (time.Time, error) {
	whoisServer, err := getWhoisServer(domain)
	if err != nil {
		return time.Time{}, err
	}

	conn, err := net.Dial("tcp", whoisServer+":43")
	if err != nil {
		return time.Time{}, err
	}
	defer conn.Close()

	fmt.Fprintf(conn, domain+"\r\n")

	response, err := ioutil.ReadAll(conn)
	if err != nil {
		return time.Time{}, err
	}

	re := regexp.MustCompile(`(?i)Expir.+?(\d{4}-\d{2}-\d{2})`)
	matches := re.FindStringSubmatch(string(response))
	if len(matches) < 2 {
		return time.Time{}, errors.New("无法解析域名到期时间")
	}

	expirationDate, err := time.Parse("2006-01-02", matches[1])
	if err != nil {
		return time.Time{}, err
	}

	return expirationDate, nil
}

func getWhoisServer(domain string) (string, error) {
	parts := strings.Split(domain, ".")
	tld := parts[len(parts)-1]

	conn, err := net.Dial("tcp", "whois.iana.org:43")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	fmt.Fprintf(conn, tld+"\r\n")

	response, err := ioutil.ReadAll(conn)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`whois:\s*(\S+)`)
	matches := re.FindStringSubmatch(string(response))
	if len(matches) < 2 {
		return "", errors.New("无法解析 whois 服务器地址")
	}

	return matches[1], nil
}

func runExternalProgram(external string, message string, method string, argsTemplate string) error {
	var cmd *exec.Cmd
	if method == "args" {
		args := strings.Split(argsTemplate, " ")
		for i, arg := range args {
			args[i] = strings.Replace(arg, "{message}", message, -1)
		}
		cmd = exec.Command(external, args...)
		fmt.Printf("运行命令: %s %v\n", external, args)
	} else {
		cmd = exec.Command(external)
		cmd.Stdin = bytes.NewBufferString(message)
	}
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("运行外部程序 %s 出错: %v\n", external, err)
		fmt.Printf("错误信息: %s\n", stderr.String())
		return fmt.Errorf("%v: %s", err, stderr.String())
	} else {
		fmt.Printf("运行外部程序 %s 成功\n", external)
		fmt.Printf("运行信息: %s\n", out.String())
	}
	return nil
}
