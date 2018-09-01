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

func waitSignal() {
	var sigChan = make(chan os.Signal, 1)
	signal.Notify(sigChan)
	for sig := range sigChan {
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
	flag.StringVar(&logTo, "logto", "stdout", "stdout or syslog")
	flag.StringVar(&faddr, "listen", ":9001", "host:port qtunnel listen on")
	flag.StringVar(&baddr, "backend", "127.0.0.1:6400", "host:port of the backend")
	flag.StringVar(&cryptoMethod, "crypto", "rc4", "encryption method")
	flag.StringVar(&secret, "secret", "secret", "password used to encrypt the data")
	flag.StringVar(&conf, "conf", "", "read connection setup from config file")
	flag.StringVar(&tag, "tag", "", "only setup the tag in config file")
	flag.BoolVar(&clientMode, "clientmode", false, "if running at client mode")
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
		var s_len = len(sections)
		for i, s := range sections {
			if len(tag) > 0 && s != tag {
				if i == s_len {
					log.Printf("can not find tag %s, exit!", tag)
					os.Exit(1)
				}
				continue
			}

			if s == "default" {
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
				t := tunnel.NewTunnel(fdr, bdr, cld, crt, set, 4096)
				log.Printf("qtunnel start from %s to %s.", fdr, bdr)
				t.Start()
			}()
		}
	} else {
		t := tunnel.NewTunnel(faddr, baddr, clientMode, cryptoMethod, secret, 4096)
		log.Println("qtunnel started.")
		go t.Start()
	}
	waitSignal()
}
