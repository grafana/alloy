package korniltsev

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/grafana/alloy/internal/component/discovery"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

var clusters = []string{
	//"dev-eu-west-3",
	"dev-us-central-0",
	//"dev-us-central-1",
	"ops-us-east-0",
	"prod-ap-south-0",
	"prod-ap-southeast-0",
	"prod-au-southeast-0",
	"prod-eu-west-0",
	"prod-gb-south-0",
	"prod-sa-east-0",
	"prod-us-central-0",
	"prod-us-central-3",
	"prod-us-central-4",
	"prod-us-central-5",
	//"prod-us-central-6",
	"prod-us-east-1",
}

func TestDiscover(t *testing.T) {

	for _, cluster := range clusters {

		cachepods(t, cluster)
	}

}

func TestCached(t *testing.T) {
	for _, cluster := range clusters {
		t.Run(cluster, func(t *testing.T) {
			testone(t, cluster)
		})

	}
}

/*
 "exports": [
    {
      "name": "targets",
      "type": "attr",
      "value": {
        "type": "array",
        "value": [
          {
            "type": "object",
            "value": [
              {
                "key": "__address__",
                "value": {
                  "type": "string",
                  "value": "10.128.0.56"
                }
              },
              {
                "key": "__meta_kubernetes_namespace",
                "value": {
                  "type": "string",
                  "value": "kube-system"
                }
              },

*/

type Export struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value struct {
		Type  string `json:"type"`
		Value []struct {
			Type  string `json:"type"`
			Value []struct {
				Key   string `json:"key"`
				Value struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"value"`
			}
		} `json:"value"`
	} `json:"value"`
}
type Response struct {
	Exports []json.RawMessage `json:"exports"`
}

//type ComponentResponse struct {
//	Components []Component `json:"components"`
//}

type Component struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	LocalID      string   `json:"localID"`
	ModuleID     string   `json:"moduleID"`
	Label        string   `json:"label"`
	ReferencesTo []string `json:"referencesTo"`
	ReferencedBy []string `json:"referencedBy"`
	Health       struct {
		State       string    `json:"state"`
		Message     string    `json:"message"`
		UpdatedTime time.Time `json:"updatedTime"`
	} `json:"health"`
	Original  string        `json:"original"`
	Arguments []interface{} `json:"arguments"`
	Exports   []interface{} `json:"exports"`
	DebugInfo []interface{} `json:"debugInfo"`
}

func testone(t *testing.T, cluster string) {
	configriver := getconfigriver(t, cluster)

	m := newmockpods(t, cluster)
	defer m.Close()

	configriver = processConfigRiver(configriver)

	cmd := startAlloy(t, string(configriver))
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	var alltargets []discovery.Target
	time.Sleep(1 * time.Second)

	for {
		_, alltargets = getComponent(t, cluster, "discovery.kubernetes", "pyroscope_kubernetes")
		if len(alltargets) > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	time.Sleep(1 * time.Second)

	//curl 'http://localhost:12345/api/v0/web/components/discovery.kubernetes.pyroscope_kubernetes'
	//u := fmt.Sprintf("http://localhost:12345/api/v0/web/components/discovery.kubernetes.%s", cluster)
	components := getComponents(t)
	for i, component := range components {
		fmt.Printf("%d %s %s\n", i, component.Name, component.Health.State)
	}
	targetmap := make(map[string]map[string]discovery.Target)
	for _, component := range components {

		if strings.Contains("discovery.relabel", component.Name) {

			name, targets := getRelabel(t, cluster, component)
			itmap := make(map[string]discovery.Target)
			for _, target := range targets {
				itmap[getTargetPQ(target)] = target
			}
			targetmap[name] = itmap
			//fmt.Printf("%+v\n", resp)
		}
	}
	doChecks(t, cluster, alltargets, targetmap)

}

func startAlloy(t *testing.T, configriver string) *exec.Cmd {
	tmpfile, _ := os.CreateTemp("", "cfg")
	tmpfile.Write([]byte(configriver))
	tmpfile.Close()

	cmd := exec.Command("/home/korniltsev/p/alloy/alloy", "run", tmpfile.Name())
	out := bytes.NewBuffer(nil)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Start()
	if err != nil {
		fmt.Print(string(out.Bytes()))
		t.Fatal("failed to start alloy")
	}
	return cmd
}

func doChecks(t *testing.T, cluster string, alltargets []discovery.Target, targetmap map[string]map[string]discovery.Target) {
	os.MkdirAll(fmt.Sprintf("data/tests/%s", cluster), 0755)
	logf, _ := os.Create(fmt.Sprintf("data/tests/%s/log.txt", cluster))
	defer logf.Close()
	for _, ittarget := range alltargets {
		relabels := make(map[string]struct{})
		all := []string{}
		for rk, rt := range targetmap {
			target := rt[getTargetPQ(ittarget)]

			if target != nil {
				all = append(all, rk)
				k := rk
				if strings.Contains(k, "sb_relabel_annotation_based") {
					k = "sb_relabel_annotation_based"
				} else if strings.Contains(k, "godeltaprof") {
					k = "godeltaprof"
				}
				if strings.Contains(k, "ebpf") || strings.Contains(k, "ebpf_local_pods") {
					k = "ebpf"
				}
				//if k == "ebpf" {
				//	continue
				//}
				relabels[k] = struct{}{}
				//relabels = append(relabels, rk)
			}
		}
		if len(relabels) > 1 {
			msg := fmt.Sprintf("%s %+v %+v\n", getTargetPQ(ittarget), relabels, all)
			_, _ = logf.WriteString(msg)
			t.Error(msg)
		}
	}
}

func getRelabel(t *testing.T, cluster string, c Component) (string, []discovery.Target) {
	name := c.Name
	label := c.Label
	return getComponent(t, cluster, name, label)
}

func getComponent(t *testing.T, cluster string, name string, label string) (string, []discovery.Target) {
	fmt.Printf("relabel %s %s\n", name, label)
	u := fmt.Sprintf("http://localhost:12345/api/v0/web/components/%s.%s", name, label)
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get %s: %w", u, err))
	}
	fmt.Printf("GET %s %d\n", u, resp.StatusCode)
	if resp.StatusCode != 200 {
		t.Fatal(fmt.Errorf("failed to get %s: %d", u, resp.StatusCode))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to read body: %s %w", u, err))
	}
	//fmt.Printf("%s\n", string(body))

	res := &Response{}
	err = json.Unmarshal(body, res)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to unmarshal %s (%s): %w", u, body, err))
	}

	output := res.Exports[0]

	e := &Export{}
	err = json.Unmarshal(output, e)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to unmarshal %s (%s): %w", u, output, err))
	}
	targes := []discovery.Target{}
	for _, s := range e.Value.Value {
		t := make(discovery.Target)
		for _, s2 := range s.Value {
			t[s2.Key] = s2.Value.Value
		}
		//svc := t["service_name"]
		targes = append(targes, t)
		//fmt.Printf("%40s %s\n", label, svc)
	}
	fmt.Printf("%s : %d\n", label, len(targes))
	ls := []string{}
	for _, targe := range targes {
		ls = append(ls, fmt.Sprintf("%s", getTargetPQ(targe)))
	}

	slices.Sort(ls)

	os.MkdirAll(fmt.Sprintf("data/tests/%s", cluster), 0755)
	ff, _ := os.OpenFile(fmt.Sprintf("data/tests/%s/relabel.", cluster)+name+"."+label+".txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	for _, l := range ls {
		ff.WriteString(fmt.Sprintf("%s\n", l))
	}
	ff.Close()

	return fmt.Sprintf("%s.%s", name, label), targes
}

func getTargetPQ(t discovery.Target) string {
	return t["__meta_kubernetes_namespace"] + "/" + t["__meta_kubernetes_pod_name"] + "/" + t["__meta_kubernetes_pod_container_name"] + "/" + t["__meta_kubernetes_pod_container_port_name"]
}

func getComponents(t *testing.T) []Component {
	u := "http://localhost:12345/api/v0/web/components"
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get %s: %w", u, err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to read body: %s %w", u, err))
	}
	fmt.Printf("%s\n", string(body))
	res := []Component{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to unmarshal %s (%s): %w", u, body, err))
	}
	return res
}

func processConfigRiver(cfg []byte) []byte {

	re := regexp.MustCompile("(?s)discovery.kubernetes \"([^\"]+)\" {[^{]*selectors {[^}]*}[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		it := re.FindAllSubmatch(b, -1)[0]
		name := string(it[1])
		template := `
discovery.kubernetes "%s" {
  api_server = "http://localhost:8003"
  role = "pod"
}
`
		repl := fmt.Sprintf(template, name)
		//fmt.Printf("%s\n", string(b))
		//fmt.Printf("%s\n", string(repl))
		//fmt.Printf("%s\n", name)
		return []byte(repl)
	})
	re = regexp.MustCompile("(?s)pyroscope\\.ebpf[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	//profile.block {
	//	enabled = false
	//}
	//profile.fgprof {
	//	enabled = false
	//}
	//profile.godeltaprof_block {
	//	enabled = true
	//}
	//profile.godeltaprof_memory {
	//	enabled = false
	//}
	//profile.godeltaprof_mutex {
	//	enabled = false
	//}
	//profile.goroutine {
	//	enabled = false
	//}
	//profile.memory {
	//	enabled = false
	//}
	//profile.mutex {
	//	enabled = false
	//}
	//profile.process_cpu {
	//	enabled = false
	//}
	re = regexp.MustCompile("(s?)rule {\\s+action = \"labeldrop\"\\s+regex = \"__meta_kubernetes[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	re = regexp.MustCompile("(?s)external_labels =[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	re = regexp.MustCompile("(?s)profile\\.(block|godeltaprof_block|godeltaprof_memory|mutex|process_cpu|godeltaprof_mutex|goroutine|memory||)[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	re = regexp.MustCompile("(?s)profiling_config [^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	re = regexp.MustCompile("(?s)tls_config[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	re = regexp.MustCompile("(?s)pyroscope\\.scrape[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	re = regexp.MustCompile("(?s)external_labels ={[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	re = regexp.MustCompile("(?s)basic_auth[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	re = regexp.MustCompile("(?s)endpoint {[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})

	re = regexp.MustCompile("(?s)pyroscope\\.write[^}]*}")
	cfg = re.ReplaceAllFunc(cfg, func(b []byte) []byte {
		return []byte("")
	})
	//fmt.Printf("%s\n", string(cfg))
	return cfg
}

func getpodsсcached(t *testing.T, cluster string) []byte {
	fname := fmt.Sprintf("data/pods/%s.json", cluster)
	data, err := os.ReadFile(fname)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to read pods json %s: %w", fname, err))
	}
	return data
}

func getconfigriver(t *testing.T, cluster string) []byte {
	bytes := getconfigyaml(t, cluster)
	//data:
	//	config.river: |
	//discovery.kubernetes "ebpf_all_pods" {
	type Config struct {
		Data struct {
			ConfigRiver string `yaml:"config.river"`
		}
	}
	var c Config
	err := yaml.Unmarshal(bytes, &c)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to unmarshal yaml: %w", err))
	}
	return []byte(c.Data.ConfigRiver)
}

func getconfigyaml(t *testing.T, cluster string) []byte {
	fname := fmt.Sprintf("data/configs/%s/v1.ConfigMap-profiler.yaml", cluster)
	data, err := os.ReadFile(fname)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to read config yaml %s: %w", fname, err))
	}
	return data
}

func cachepods(t *testing.T, cluster string) {
	proxy := newproxy(t, cluster)

	url := "http://localhost:8001/api/v1/pods"
	fmt.Printf("GET %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get %s: %w", url, err))
	}
	dst := fmt.Sprintf("data/pods/%s.json", cluster)

	// create parent if not exists
	os.MkdirAll("data/pods", 0755)
	f, err := os.Create(dst)
	if err != nil {
		t.Fatal(fmt.Errorf("failed to create %s: %w", dst, err))
	}

	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	fmt.Printf("%d %d\n", resp.StatusCode, resp.ContentLength)

	defer proxy.Close()

}

func newproxy(t *testing.T, cluster string) *proxy {
	output, err := exec.Command("/home/korniltsev/apps/bin/kubectx", cluster).CombinedOutput()
	if err != nil {
		fmt.Printf("kubectx %s : %s", cluster, output)
		t.Fatal(fmt.Errorf("failed to set kubectx %s: %w", cluster, err))
	}
	fmt.Printf("kubectx %s : %s", cluster, output)
	cmd := exec.Command("/home/korniltsev/apps/bin/kubectl", "proxy")
	pipe, _ := cmd.StdoutPipe()
	err = cmd.Start()
	if err != nil {
		t.Fatal(fmt.Errorf("failed to start kubectl proxy: %w", err))
	}
	// read until "Starting to serve on"
	scanner := bufio.NewScanner(pipe)
	for {
		scan := scanner.Scan()
		if !scan {
			break
		}
		t := scanner.Text()
		if strings.Contains(t, "Starting to serve on") {
			fmt.Printf("proxy started %s\n", t)
			break
		}
	}

	return &proxy{cmd: cmd}
}

type proxy struct {
	cmd *exec.Cmd
}

func (p *proxy) Close() error {
	_ = p.cmd.Process.Kill()
	return nil
}

func newmockpods(t *testing.T, cluster string) *mockpods {
	res := &mockpods{cluster: cluster}

	server := &http.Server{
		Addr:    "127.0.0.1:8003",
		Handler: http.HandlerFunc(res.handler),
	}
	res.server = server
	res.data = getpodsсcached(t, cluster)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			fmt.Printf("server.ListenAndServe %v\n", err)
		}
	}()
	return res
}

type mockpods struct {
	cluster string
	server  *http.Server
	data    []byte
}

func (m *mockpods) handler(writer http.ResponseWriter, request *http.Request) {
	fmt.Printf("GET %s\n", request.URL)
	writer.Header().Set("Content-Type", "application/json")
	data := m.data
	writer.Write(data)
}

func (m *mockpods) Close() {
	m.server.Close()
}
