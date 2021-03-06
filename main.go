package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
	process(t)
	systray.Run(onReady(t), onExit)
}

func process(c *config) {
	if c.Pwd != "" {
		if err := os.Chdir(c.Pwd); err != nil {
			log.Errorf("ERR: %s", err)
			return
		}
	}
	for _, g := range c.Generators {
		cmds := g.Find.cmd()
		bout := &bytes.Buffer{}
		berr := &bytes.Buffer{}
		if err := pipeline(bout, berr, cmds...); err != nil {
			log.Errorf("could not generate: %+v", g.Find)
			sc := bufio.NewScanner(berr)
			for sc.Scan() {
				log.Errorf("ERR: %s", sc.Text())
			}
			continue
		}
		sc := bufio.NewScanner(bout)
		for sc.Scan() {
			log.Infof("found: %s", sc.Text())
			c.Items = append(c.Items, &item{
				Name:     sc.Text(),
				Template: g.Template,
				Vars: map[string]interface{}{
					"svc": sc.Text(),
				},
			})
		}
	}
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
	}
}

type config struct {
	Title      string
	Icon       string
	Pwd        string
	Items      []*item
	Templates  []*item
	Generators []*generator
}

type generator struct {
	Name     string
	Find     *command
	Template string
	Vars     map[string]interface{}
}

type command struct {
	Cmd  string
	Args []string
	Pipe *command
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
		i.miStart.Disable()
		i.miStop.Enable()
	} else {
		i.miStart.Enable()
		i.miStop.Disable()
	}
	i.mi.SetTitle(title(i.Name, i.running))
}

type actiontype int

const (
	actionCheckStatus actiontype = 0
	actionStart       actiontype = 1
	actionStop        actiontype = 2
)

type event struct {
	item string
	typ  actiontype // false is just to update status
}

func pipeline(stdout io.Writer, stderr io.Writer, stack ...*exec.Cmd) error {
	pipes := make([]*io.PipeWriter, len(stack)-1)
	i := 0
	for ; i < len(stack)-1; i++ {
		inPipe, outPipe := io.Pipe()
		stack[i].Stdout = outPipe
		stack[i].Stderr = stderr
		stack[i+1].Stdin = inPipe
		pipes[i] = outPipe
	}
	stack[i].Stdout = stdout
	stack[i].Stderr = stderr
	return call(stack, pipes)
}

func call(stack []*exec.Cmd, pipes []*io.PipeWriter) error {
	var err error
	if stack[0].Process == nil {
		if err = stack[0].Start(); err != nil {
			return err
		}
	}
	if len(stack) > 1 {
		if err = stack[1].Start(); err != nil {
			return err
		}
		defer func() {
			if err == nil {
				pipes[0].Close()
				err = call(stack[1:], pipes[1:])
			}
		}()
	}
	return stack[0].Wait()
}

func (c *command) isOK() bool {
	cmds := c.cmd()
	bout := &bytes.Buffer{}
	berr := &bytes.Buffer{}
	err := pipeline(bout, berr, cmds...)
	if err != nil {
		log.Debugf("isOK - error: %s", err.Error())
		log.Debugf("isOK - stdout: %s", bout.String())
		log.Debugf("isOK - stderr: %s", berr.String())
	}
	return err == nil
}

func (c *command) cmd() []*exec.Cmd {
	log.Debugf("lookup command: %s, args: %v", c.Cmd, c.Args)
	cmd := exec.Command(c.Cmd, c.Args...)
	if c.Pipe != nil {
		log.Debugf("pipe - cmd: %s, args: %v", c.Pipe.Cmd, c.Pipe.Args)
		inner := c.Pipe.cmd()
		return append([]*exec.Cmd{cmd}, inner...)
	}
	return []*exec.Cmd{cmd}
}

func (c *command) do() error {
	log.Infof("running command: %s, args: %v", c.Cmd, c.Args)
	cmds := c.cmd()
	return pipeline(os.Stdout, os.Stderr, cmds...)
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
	if template == nil {
		return nil
	}
	ret := &command{
		Cmd:  applyTemplateToString(template.Cmd, vars),
		Args: applyTemplateToSlice(template.Args, vars),
		Pipe: applyTemplateToCommand(template.Pipe, vars),
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
		if c.Title == "" {
			c.Title = "SVCs"
		}
		if c.Icon != "" {
			b, err := ioutil.ReadFile(c.Icon)
			if err != nil {
				panic(err)
			}
			systray.SetIcon(b)
		}
		systray.SetTitle(c.Title)
		systray.SetTooltip("Service tray")

		events := make(chan event)

		for _, v := range c.Items {
			// TODO allow items without checks. Just set result to "UNKNOWN"
			if v.Check == nil {
				log.Errorf("invalid item: %+v", v)
			}
			v.running = v.Check.isOK()
			v.mi = systray.AddMenuItem(title(v.Name, v.running), "tooltip")
			v.miStart = v.mi.AddSubMenuItem("Start", "start the ting")
			v.miStop = v.mi.AddSubMenuItem("Stop", "stop the ting")
			// drain all click-channels into a single channel, 'agg'
			go func(k string, mi *systray.MenuItem) {
				// for _ = range mi.ClickedCh {
				for range mi.ClickedCh {
					events <- event{item: k, typ: actionStart}
				}
			}(v.Name, v.miStart)
			go func(k string, mi *systray.MenuItem) {
				// for _ = range mi.ClickedCh {
				for range mi.ClickedCh {
					events <- event{item: k, typ: actionStop}
				}
			}(v.Name, v.miStop)
			v.redraw()
		}
		mStartAll := systray.AddMenuItem("Start All", "Start all")
		go func() {
			for range mStartAll.ClickedCh {
				for _, v := range c.Items {
					events <- event{item: v.Name, typ: actionStart}
				}
			}
		}()
		mStopAll := systray.AddMenuItem("Stop All", "Stop all")
		go func() {
			for range mStopAll.ClickedCh {
				for _, v := range c.Items {
					events <- event{item: v.Name, typ: actionStop}
				}
			}
		}()
		systray.AddSeparator()
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
					events <- event{item: v.Name, typ: actionCheckStatus}
				}
			}
		}()
		for {
			select {
			case action := <-events:
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
				if runningCount < 1 {
					mStopAll.Disable()
				} else {
					mStopAll.Enable()
				}
				if runningCount == len(c.Items) {
					mStartAll.Disable()
				} else {
					mStartAll.Enable()
				}
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
