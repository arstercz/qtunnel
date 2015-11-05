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

func main() {
	var faddr, baddr, cryptoMethod, secret, logTo, conf string
	var clientMode, daemon bool
	flag.StringVar(&logTo, "logto", "stdout", "stdout or syslog")
	flag.StringVar(&faddr, "listen", ":9001", "host:port qtunnel listen on")
	flag.StringVar(&baddr, "backend", "127.0.0.1:6400", "host:port of the backend")
	flag.StringVar(&cryptoMethod, "crypto", "rc4", "encryption method")
	flag.StringVar(&secret, "secret", "secret", "password used to encrypt the data")
	flag.StringVar(&conf, "conf", "", "read connection setup from config file")
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
			log.Printf("read error from %s file", conf)
			os.Exit(1)
		}
		sections := c.GetSections()
		for _, s := range sections {
			if s == "default" {
				continue
			}
			fdr, _ := c.GetString(s, "faddr")
			bdr, _ := c.GetString(s, "baddr")
			cld, _ := c.GetBool(s, "clientmode")
			crt, _ := c.GetString(s, "cryptoMethod")
			set, _ := c.GetString(s, "secret")

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
