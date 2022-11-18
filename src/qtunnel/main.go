package main

import (
	"flag"
	"goconfig"
	"godaemon"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"tunnel"
	"time"
	"net"
)

func isTagInSection(sections []string, tag string) bool {
	if tag == "" {
		return false
	}

	for _, v := range sections {
		if v == tag {
			return true
		}
	}

	return false
}

func waitSignal() {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan)
	for sig := range sigChan {
		// ignore SIGURG, read from https://github.com/golang/go/issues/38290
		if sig == syscall.SIGURG {
			continue
		}

		if sig == syscall.SIGINT || sig == syscall.SIGTERM {
			log.Printf("terminated by signal %v\n", sig)
			return
		} else {
			log.Printf("received signal: %v, ignore\n", sig)
		}
	}
}

func check_port(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func main() {
	var faddr, baddr, cryptoMethod, secret, logTo, conf, tag string
	var clientMode, daemon bool
	var buffer uint
	var timeOut time.Duration

	flag.StringVar(&logTo, "logto", "stdout", "stdout or syslog")
	flag.StringVar(&faddr, "listen", ":9001", "host:port qtunnel listen on")
	flag.StringVar(&baddr, "backend", "127.0.0.1:6400", "host:port of the backend")
	flag.StringVar(&cryptoMethod, "crypto", "rc4", "encryption method")
	flag.StringVar(&secret, "secret", "secret", "password used to encrypt the data")
	flag.StringVar(&conf, "conf", "", "read connection setup from config file")
	flag.StringVar(&tag, "tag", "", "only setup the tag in config file")
	flag.UintVar(&buffer, "buffer", 4096, "tunnel buffer size")
	flag.BoolVar(&clientMode, "clientmode", false, "if running at client mode")
	flag.DurationVar(&timeOut, "timeout", 30 * time.Minute, "connection read deadline time limit")
	flag.BoolVar(&daemon, "daemon", false, "running in daemon mode")
	flag.Parse()

	log.SetOutput(os.Stdout)
	if logTo == "syslog" {
		w, err := syslog.New(syslog.LOG_INFO, "qtunnel")
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(w)
	}
	CurDir, _ := os.Getwd()

	if daemon == true {
		godaemon.MakeDaemon(&godaemon.DaemonAttr{})
	}
	// start from config file for multi-front-port
	if len(conf) > 0 {
		if match, _ := regexp.MatchString("^[^/]", conf); match {
			conf = CurDir + "/" + conf
		}
		c, err := goconfig.ReadConfigFile(conf)
		if err != nil {
			log.Println("read error from %s file", conf)
			os.Exit(1)
		}
		sections := c.GetSections()
		if !isTagInSection(sections, tag) {
			log.Printf("can not find tag %s, exit!", tag)
			os.Exit(1)
		}
		for _, s := range sections {
			if s == "default" || (s != "" && s != tag) {
				continue
			}

			fdr, err := c.GetString(s, "faddr")
			bdr, err := c.GetString(s, "baddr")
			cld, err := c.GetBool(s, "clientmode")
			crt, err := c.GetString(s, "cryptoMethod")
			set, err := c.GetString(s, "secret")
			if (err != nil) {
				log.Fatalln("qtunnel config error with tag: %s", tag)
				os.Exit(1)
			}

			if check_port(fdr) {
				log.Printf("qtunnel already bind %s", fdr)
				if (len(tag) > 0) {
					os.Exit(1)
				} else {
					continue
				}
			}

			go func() {
				t := tunnel.NewTunnel(fdr, bdr, cld, crt, set, uint32(buffer), timeOut)
				log.Printf("qtunnel start from %s to %s.", fdr, bdr)
				t.Start()
			}()
		}
	} else {
		t := tunnel.NewTunnel(faddr, baddr, clientMode, cryptoMethod, secret, uint32(buffer), timeOut)
		log.Println("qtunnel started.")
		go t.Start()
	}
	waitSignal()
}
