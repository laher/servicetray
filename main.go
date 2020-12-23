package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os/exec"
	"text/template"
	"time"

	"github.com/apex/log"
	"github.com/getlantern/systray"

	"gopkg.in/yaml.v2"
)

func main() {
	var configFile = flag.String("config", "servicetray.yml", "configuration file")
	flag.Parse()
	data, err := ioutil.ReadFile(*configFile)
	if err != nil {
		panic(err)
	}
	t := &config{}
	err = yaml.Unmarshal(data, t)
	if err != nil {
		panic(err)
	}
	systray.Run(onReady(t), onExit)
}

type config struct {
	Title     string
	Items     []*item
	Templates []*item
}

type command struct {
	Cmd  string
	Args []string
}

type item struct {
	Name  string
	Start *command
	Stop  *command
	Check *command

	mi      *systray.MenuItem
	miStart *systray.MenuItem
	miStop  *systray.MenuItem
	running bool

	Template string
	Vars     map[string]interface{}
}

func (i *item) redraw() {
	if i.running {
		i.miStart.Hide()
		i.miStop.Show()
	} else {
		i.miStart.Show()
		i.miStop.Hide()
	}
	i.mi.SetTitle(title(i.Name, i.running))
}

type actiontype int

const (
	actionCheckStatus actiontype = 0
	actionStart       actiontype = 1
	actionStop        actiontype = 2
)

type action struct {
	item string
	typ  actiontype // false is just to update status
}

func (c *command) isOK() bool {
	log.Debugf("running command: %s, args: %v", c.Cmd, c.Args)
	cmd := exec.Command(c.Cmd, c.Args...)
	return cmd.Run() == nil
}

func (c *command) do() error {
	log.Debugf("running command: %s, args: %v", c.Cmd, c.Args)
	cmd := exec.Command(c.Cmd, c.Args...)
	return cmd.Run()
}

func applyTemplateToString(tpl string, vars map[string]interface{}) string {
	t := template.Must(template.New("tpl").Parse(tpl))
	b := &bytes.Buffer{}
	err := t.Execute(b, vars)
	if err != nil {
		log.Warnf("executing template: %v", err)
		return tpl
	}
	return b.String()
}

func applyTemplateToSlice(in []string, vars map[string]interface{}) []string {
	ret := make([]string, 0, len(in))
	for _, i := range in {
		ret = append(ret, applyTemplateToString(i, vars))
	}
	return ret
}

func applyTemplateToCommand(template *command, vars map[string]interface{}) *command {
	ret := &command{
		Cmd:  applyTemplateToString(template.Cmd, vars),
		Args: applyTemplateToSlice(template.Args, vars),
	}
	return ret
}

func applyTemplate(template *item, entry *item) {
	entry.Start = applyTemplateToCommand(template.Start, entry.Vars)
	entry.Stop = applyTemplateToCommand(template.Stop, entry.Vars)
	entry.Check = applyTemplateToCommand(template.Check, entry.Vars)
}

func onReady(c *config) func() {
	return func() {
		systray.SetTitle(c.Title)
		systray.SetTooltip("Service tray")

		agg := make(chan action)

		for _, v := range c.Items {
			if v.Template != "" {
				found := false
				for _, t := range c.Templates {
					if v.Template == t.Name {
						applyTemplate(t, v)
						found = true
					}
				}
				if !found {
					log.Errorf("template not found: %s", v.Template)
				}
			}
			if v.Check == nil {
				log.Errorf("invalid item: %+v", v)

			}
			v.running = v.Check.isOK()
			v.mi = systray.AddMenuItem(title(v.Name, v.running), "tooltip")
			v.miStart = v.mi.AddSubMenuItem("start", "start the ting")
			v.miStop = v.mi.AddSubMenuItem("stop", "stop the ting")
			// drain all click-channels into a single channel, 'agg'
			go func(k string, mi *systray.MenuItem) {
				// for _ = range mi.ClickedCh {
				for _ = range mi.ClickedCh {
					agg <- action{item: k, typ: actionStart}
				}
			}(v.Name, v.miStart)
			go func(k string, mi *systray.MenuItem) {
				// for _ = range mi.ClickedCh {
				for _ = range mi.ClickedCh {
					agg <- action{item: k, typ: actionStop}
				}
			}(v.Name, v.miStop)
			v.redraw()
		}
		mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
		go func() {
			<-mQuit.ClickedCh
			log.Info("Requesting quit")
			systray.Quit()
			log.Info("Finished quitting")
		}()

		go func() {
			for {
				// every 5 seconds, check status
				time.Sleep(time.Second * 5)
				for _, v := range c.Items {
					agg <- action{item: v.Name, typ: actionCheckStatus}
				}
			}
		}()
		for {
			select {
			case action := <-agg:
				log.Debugf("received event: %+v", action)
				runningCount := 0
				for _, item := range c.Items {
					if item.Name == action.item {
						switch action.typ {
						case actionStop:
							log.Infof("stopping: %+v", item)
							if err := item.Stop.do(); err != nil {
								log.Warnf("couldnt stop: %v", err)
								item.running = item.Check.isOK()
							} else {
								item.running = false
							}
						case actionStart:
							log.Infof("starting: %+v", item)
							if err := item.Start.do(); err != nil {
								log.Warnf("couldnt start: %v", err)
								item.running = item.Check.isOK()
							} else {
								item.running = true
							}
						case actionCheckStatus:
							item.running = item.Check.isOK()
						}
						item.redraw()
					}
					if item.running {
						runningCount++
					}
				}
				systray.SetTitle(title(c.Title, runningCount > 0) + fmt.Sprintf(" [%d/%d]", runningCount, len(c.Items)))
			}
		}
	}
}

func title(name string, running bool) string {
	status := "🖣"
	if running {
		status = "🖢"
	}
	return fmt.Sprintf("%s %s", status, name)
}

func onExit() {
	// clean up here
}
