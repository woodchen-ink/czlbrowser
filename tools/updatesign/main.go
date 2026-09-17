// updatesign 生成与签名应用更新清单 latest.json。
//
//	go run ./tools/updatesign keygen
//	    生成 Ed25519 密钥对：私钥种子写入 GitHub Secret UPDATE_SIGNING_KEY，
//	    公钥写入 backend/internal/update/pubkey.go。
//	go run ./tools/updatesign sign -in manifest.json -out latest.json
//	    从环境变量 UPDATE_SIGNING_KEY 读取私钥种子，输出签名信封。
//
// 信封格式与 backend/internal/update/manifest.go 的 Envelope 保持一致：
// manifest 字段为清单原始字节的 base64，签名针对这段原始字节，避免 JSON 规范化差异。
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type envelope struct {
	Manifest  string `json:"manifest"`
	Signature string `json:"signature"`
}

func main() {
	if len(os.Args) < 2 {
		fail("用法: updatesign keygen | sign -in manifest.json -out latest.json")
	}
	switch os.Args[1] {
	case "keygen":
		keygen()
	case "sign":
		sign(os.Args[2:])
	default:
		fail("未知子命令: " + os.Args[1])
	}
}

func keygen() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fail("生成密钥失败: " + err.Error())
	}
	fmt.Println("私钥种子 (UPDATE_SIGNING_KEY，勿泄露):")
	fmt.Println(base64.StdEncoding.EncodeToString(priv.Seed()))
	fmt.Println("公钥 (写入 backend/internal/update/pubkey.go):")
	fmt.Println(base64.StdEncoding.EncodeToString(pub))
}

func sign(args []string) {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	in := fs.String("in", "", "清单 JSON 路径")
	out := fs.String("out", "", "输出信封路径")
	_ = fs.Parse(args)
	if *in == "" || *out == "" {
		fail("sign 需要 -in 与 -out")
	}

	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("UPDATE_SIGNING_KEY")))
	if err != nil || len(seed) != ed25519.SeedSize {
		fail("环境变量 UPDATE_SIGNING_KEY 缺失或不是 32 字节 base64 种子")
	}
	priv := ed25519.NewKeyFromSeed(seed)

	payload, err := os.ReadFile(*in)
	if err != nil {
		fail("读取清单失败: " + err.Error())
	}
	if !json.Valid(payload) {
		fail("清单不是合法 JSON")
	}

	data, err := json.MarshalIndent(envelope{
		Manifest:  base64.StdEncoding.EncodeToString(payload),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload)),
	}, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0644); err != nil {
		fail("写入信封失败: " + err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
