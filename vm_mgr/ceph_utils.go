package vm_mgr

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
)

func GetMonHost() (mon_hosts string, err error) {
	cmd := exec.Command("ceph", "mon", "dump", "--format=json")
	// var out bytes.Buffer
	// cmd.Stdout = &out
	// cmd.Stderr = &out
	// err := cmd.Run()
	// if err != nil {
	// 	fmt.Println("ERR")
	// }
	// fmt.Printf("out:%s", out.String())

	result, _ := cmd.Output()
	fmt.Println(reflect.TypeOf(result))
	is_json := json.Valid(result)
	if is_json == true {
		fmt.Println("True")
	}
	type MonHost struct {
		Rank uint32 `json: "rank"`
		Name string `json: "name"`
		Addr string `json: "addr"`
	}
	type CephMon struct {
		Epoch    int `json:"epoch"`
		Fsid     string `json: "fsid"`
		Modified string `json: "modified"`
		Created  string `json: "created"`
		Mons     []MonHost `json: "mons"`
		Quorum   []uint `json: "quorum"`
	}
	ceph_mons := new(CephMon)
	err = json.Unmarshal(result, ceph_mons)
	if err != nil {
		fmt.Println(err)
		fmt.Println("ceph mon dump failed")
	}
	fmt.Printf("ceph: %+v\n", ceph_mons)
	for idx, host := range ceph_mons.Mons {
		if idx == 0 {
			mon_hosts = fmt.Sprintf("%s", host.Addr)
			continue
		}
		mon_hosts = fmt.Sprintf("%s,%s", host.Addr, mon_hosts)
	}
	return
}

func JsonFormatTest() {
	result := []byte(`{"epoch":1,"fsid":"756d71a5-70eb-4586-bc2f-6aed24dea531","modified":"2018-01-08 11:39:04.506632","created":"2018-01-08 11:39:04.506632","features":{"persistent":["kraken","luminous"],"optional":[]},"mons":[{"rank":0,"name":"10.70.238.61","addr":"10.70.238.61:6789/0","public_addr":"10.70.238.61:6789/0"}],"quorum":[0]}`)
	fmt.Printf("%s", result)
	type MonHost struct {
		rank int
		name string
		addr string
	}
	type CephMon struct {
		Epoch    uint
		Fsid     string
		Modified string
		created  string
		Features string
		Mons     []MonHost
		Quorum   string
	}
	var ceph_mons = new(CephMon)
	err := json.Unmarshal(result, ceph_mons)
	if err != nil {
		fmt.Println("ERR")
	}
	fmt.Printf("ceph: %s", ceph_mons)
}

/*
* Only parse MonHost addr from Json fromat output
 */
func GetOnlyMonHost() {
	cmd := exec.Command("ceph", "mon", "dump", "--format=json")
	result, _ := cmd.Output()
	type MonHost struct {
		Addr string
	}
	type Mons struct {
		Mons []MonHost
	}
	ceph_mons := new(Mons)
	err := json.Unmarshal(result, ceph_mons)
	if err != nil {
		fmt.Println("ERR")
	}

	fmt.Println("mons:", ceph_mons.Mons)

	for _, host := range ceph_mons.Mons {
		fmt.Println(host.Addr)
		fmt.Println(reflect.TypeOf(host.Addr))
	}
}

func SimpleJson() {
	test := []byte(`{"name": "test1", "vol": "uuid1"}`)
	type TestJson struct {
		Name string `json:"name"`
		Vol  string `json:"vol"`
	}
	var itf TestJson
	err := json.Unmarshal(test, &itf)
	if err != nil {
		fmt.Println("ERR")
	}
	fmt.Printf("%+v", itf)
}

func LocalCall() {
	GetMonHost()
	// JsonFormatTest()
	// SimpleJson()
	// GetOnlyMonHost()
}
