/*
QQWry.dat里面全部采用了little-endian字节序
文件结构说明：
http://lumaqq.linuxsir.org/article/qqwry_format_detail.html
*/

package main

import (
	zh "code.google.com/p/go.text/encoding/simplifiedchinese"
	"code.google.com/p/go.text/transform"
	"encoding/binary"
	"encoding/json"
	"flag"
	"regexp"
	"fmt"
	//"github.com/rchunping/ip2location-qqwry/go-iconv" //iconv
	"log"
	"bytes"
	"net"
	"net/http"
	//"net/url"
	"io/ioutil"
	"lib/daemon"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
)

type tIp2LocationService struct {
	w http.ResponseWriter
	r *http.Request
}
type tIp2LocationReq struct {
	ip string //查询的ip
}
type tIp2LocationResp struct {
	ok      bool
	ip      string
	area_name 	string
	area_desc 		string
	op_name 	string
	op_desc    string
}

var queryLength = 2
var queryPool chan tIp2LocationReq
var recodePool chan tIp2LocationResp
var queryMutex sync.RWMutex

func (this *tIp2LocationService) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	ipStr := r.FormValue("ip")

	//auto detect...
	if ipStr == "" {
		ipStr = r.Header.Get("X-Real-IP") // nginx
	}

	if ipStr == "" {
		ipStr = r.Header.Get("X-Forwarded-For") // proxy

		// 保留ip
		// A类:10.0.0.0-10.255.255.255
		// B类:172.16.0.0-172.31.255.255
		// C类:192.168.0.0-192.168.255.255

		// 可能经过多个代理，结构是这样的 ip1,ip2,...
		if strings.Index(ipStr, ",") > -1 {
			for _, _ip := range strings.Split(ipStr, ",") {
				_ip = strings.Trim(_ip, " ")
				if strings.Index(_ip, "10.") == 0 || strings.Index(_ip, "192.168.") == 0 {
					continue
				}
				_skip := false
				for bi := 16; bi <= 32; bi++ {
					if strings.Index(_ip, fmt.Sprintf("172.%d.", bi)) == 0 {
						_skip = true
						break
					}
				}

				if _skip {
					continue
				}

				// valid ip
				ipStr = _ip
				break
			}
		}
	}

	if ipStr == "" {
		ipStr = r.Header.Get("Client-Ip") // ??
	}

	if ipStr == "" {
		_ra := r.RemoteAddr // IP:port

		_ras := strings.Split(_ra, ":")
		ipStr = _ras[0]
	}

	//ipStr = "192.168.1.66"

	//log.Printf("%#v",ipStr)

	if ipStr == "" {
		//log.Printf("RemoteAddr: %#v\n",r.RemoteAddr)
		//log.Printf("%#v",r.Header)
		//10.0.0.0/8, 172.16.0.0/12 or 192.168.0.0/16
	}

	// query -> chanA ,,,then,,,, chanB -> result
	// must be locked.
	queryMutex.Lock()
	queryPool <- tIp2LocationReq{
		ip: ipStr,
	}

	record := <-recodePool
	queryMutex.Unlock()

	//addr, err := net.ResolveIPAddr("ip4", "google.com")
	//ip := net.ParseIP(ipStr) //"202.101.172.36")
	//var addr net.IPAddr
	//addr.IP = ip.To4() // := &net.IPAddr{IP:ip}
	//log.Printf("%#v",addr)

	//log.Printf("%#v",record)
	//return

	log.Printf("ip:%s area_desc:%s area_name:%s op_desc:%s op_name:%s", ipStr, record.area_desc,record.area_name, record.op_desc,record.op_name)

	ret := map[string]interface{}{
		"ok":      record.ok,
		"ip":      ipStr,
		"area_name":    record.area_name,
		"area_desc": record.area_desc,
		"op_name":    record.op_name,
		"op_desc":    record.op_desc,
	}

	js, _ := json.Marshal(ret)
	this.w.Write([]byte(js))
	//this.w.Write([]byte(",\"geo\":"))
	//this.w.Write([]byte("null}"))
	return

}

type tIp2LocationServiceJSONP struct {
}

// 封装成JSONP
// 如果没有callback参数，则还是json输出
func (srv *tIp2LocationServiceJSONP) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	cb := r.FormValue("callback")

	//output
	ot := r.FormValue("ot")

	//需要有其他格式输出，禁止jsonp分装
	if ot != "" && ot != "jsonp" {
		cb = ""
	}

	if cb != "" {
		w.Header().Set("Content-Type", "text/javascript;charset=UTF-8")
		w.Write([]byte(cb))
		w.Write([]byte("("))
	} else {

		w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	}
	goSrv := &tIp2LocationService{
		w: w,
		r: r,
	}
	goSrv.ServeHTTP(w, r)

	if cb != "" {
		w.Write([]byte(")"))
	}
}

func startQueryService(dbfile string) {

	_file, err := os.Open(dbfile)

	if err != nil {
		log.Fatal(err)
	}
	fileInfo, _ :=_file.Stat();

	buf := make([]byte, fileInfo.Size())
	_file.Read(buf);
	file:=bytes.NewReader(buf);

	// header
	file.ReadAt(buf[0:8], 0)

	//log.Printf("%#v",buf[:8])

	indexStart := int64(binary.LittleEndian.Uint32(buf[0:4]))
	indexEnd := int64(binary.LittleEndian.Uint32(buf[4:8]))

	log.Printf("Index range: %d - %d", indexStart, indexEnd)

	for {

		req, eor := <-queryPool

		if !eor {
			log.Fatal("empty query.")
		}

		//log.Printf("%#v",req)

		ip := net.ParseIP(req.ip)
		ip4 := make([]byte, 4)
		ip4 = ip.To4() // := &net.IPAddr{IP:ip}
		//log.Printf("IP4: %#v",ip4)

		//log.Printf("%#v",req.ip)

		//二分法查找
		maxLoop := int64(32)
		head := indexStart //+ 8
		tail := indexEnd   //+ 8

		//是否找到
		got := false
		rpos := int64(0)

		for ; maxLoop >= 0 && len(ip4) == 4; maxLoop-- {
			idxNum := (tail - head) / 7
			pos := head + int64(idxNum/2)*7

			//pos += maxLoop*7

			file.ReadAt(buf[0:7], pos)

			// startIP
			_ip := binary.LittleEndian.Uint32(buf[0:4])

			//log.Printf("%d - INs:%d POS:%d %#v %d.%d.%d.%d",maxLoop,idxNum,pos,buf[0:7],_ip&0xff,_ip>>8&0xff,_ip>>16&0xff,_ip>>24&0xff)

			//记录位置
			_buf := append(buf[4:7], 0x0) // 3byte + 1byte(0x00)
			rpos = int64(binary.LittleEndian.Uint32(_buf))
			//log.Printf("%d %#v",rpos,_buf)

			file.ReadAt(buf[0:4], rpos)

			_ip2 := binary.LittleEndian.Uint32(buf[0:4])

			//log.Printf("IP_END:%#v %d.%d.%d.%d",buf[0:4],_ip2&0xff,_ip2>>8&0xff,_ip2>>16&0xff,_ip2>>24&0xff)

			//查询的ip被转成大端了
			_ipq := binary.BigEndian.Uint32(ip4)

			//log.Printf("%d - IP_START: %#v",maxLoop,_ip)
			//log.Printf("%d - IP_END  : %#v",maxLoop,_ip2)
			//log.Printf("%d - IP_QUERY: %#v",maxLoop,_ipq)

			if _ipq > _ip2 {
				head = pos
				continue
			}

			if _ipq < _ip {
				tail = pos
				continue
			}

			// got

			got = true

			break

		}

		//log.Printf("GOT: %#v POS: %d", got, rpos)

		loc := tIp2LocationResp{
			ok:      false,
			ip:      req.ip,
			area_name:    "",
			area_desc: "",
			op_desc:    "",
			op_name:    "",
		}
		if got {
			_loc := getIpLocation(file, rpos)


			// //cd, err := iconv.Open("UTF-8//IGNORE", "GBK")
			// if err != nil {

			// 	// 编码问题，不报错，直接返回false
			// 	//panic("iconv.Open failed!")

			// 	recodePool <- loc

			// 	continue

			// }

			// defer cd.Close()


			//log.Printf("ICONV: %#v %#v",xx,ex)


			var tr *transform.Reader
			tr = transform.NewReader(strings.NewReader(_loc.area_desc), zh.GBK.NewDecoder())

			if s, err := ioutil.ReadAll(tr); err == nil {
				loc.area_desc = string(s)
			}

			tr = transform.NewReader(strings.NewReader(_loc.op_desc), zh.GBK.NewDecoder())

			if s, err := ioutil.ReadAll(tr); err == nil {
				loc.op_desc = string(s)
			}

			loc.ok = _loc.ok
			//{{{
			re := regexp.MustCompile("(铁通|电信|联通|移动|网通)")
			loc.op_name = re.FindString(loc.op_desc);
			re_area := regexp.MustCompile("(北京|天津|河北|山西|内蒙|辽宁|吉林|黑龙|上海|江苏|浙江|安徽|福建|江西|山东|河南|湖北|湖南|广东|广西|海南|重庆|四川|贵州|云南|西藏|陕西|甘肃|青海|宁夏|新疆|香港|澳门|台湾)")
			loc.area_name= re_area.FindString(loc.area_desc);
			//}}}

		}

		//log.Printf("LOC: %#v", loc)

		recodePool <- loc

	}

}

func getIpLocation(file *bytes.Reader, offset int64) (loc tIp2LocationResp) {

	buf := make([]byte, 1024)

	file.ReadAt(buf[0:1], offset+4)

	mod := buf[0]

	//log.Printf("C1 FLAG: %#v", mod)

	descOffset := int64(0)
	op_descOffset := int64(0)

	if 0x01 == mod {
		descOffset = _readLong3(file, offset+5)
		//log.Printf("Redirect to: %#v",descOffset);

		file.ReadAt(buf[0:1], descOffset)

		mod2 := buf[0]

		//log.Printf("C2 FLAG: %#v", mod2)

		if 0x02 == mod2 {
			loc.area_desc = _readString(file, _readLong3(file, descOffset+1))
			op_descOffset = descOffset + 4
		} else {
			loc.area_desc = _readString(file, descOffset)
			op_descOffset = descOffset + int64(len(loc.area_desc)) + 1
			// op_descOffset = file.Seek(0,1) // got the pos
			// log.Printf("cPOS: %#v aPOS: %#v err: %#v",descOffset,op_descOffset,err3)

		}

		loc.op_desc = _readArea(file, op_descOffset)

	} else if 0x02 == mod {
		loc.area_desc = _readString(file, _readLong3(file, offset+5))
		loc.op_desc = _readArea(file, offset+8)
	} else {
		loc.area_desc = _readString(file, offset+4)
		op_descOffset = offset + 4 + int64(len(loc.area_desc)) + 1
		//op_descOffset,_ = file.Seek(0,1)

		loc.op_desc = _readArea(file, op_descOffset)
	}

	loc.ok = true

	return
}
func _readArea(file *bytes.Reader, offset int64) string {
	buf := make([]byte, 4)

	file.ReadAt(buf[0:1], offset)

	mod := buf[0]

	//log.Printf("A FLAG: %#v", mod)

	if 0x01 == mod || 0x02 == mod {
		op_descOffset := _readLong3(file, offset+1)
		if op_descOffset == 0 {
			return ""
		} else {
			return _readString(file, op_descOffset)
		}
	}
	return _readString(file, offset)
}

func _readLong3(file *bytes.Reader, offset int64) int64 {
	buf := make([]byte, 4)
	file.ReadAt(buf, offset)
	buf[3] = 0x00

	return int64(binary.LittleEndian.Uint32(buf))
}

func _readString(file *bytes.Reader, offset int64) string {

	buf := make([]byte, 1024)
	got := int64(0)

	for ; got < 1024; got++ {
		file.ReadAt(buf[got:got+1], offset+got)

		if buf[got] == 0x00 {
			break
		}
	}

	return string(buf[0:got])
}

func main() {

	var dbfile, bindaddr *string
	var filename bytes.Buffer
	dbfile = flag.String("f", "", "database file")
	bindaddr = flag.String("b", "0.0.0.0:9002", "listen port")
	flag.Parse()
	if(len(*dbfile)==0){
		_file, _ := exec.LookPath(os.Args[0])
		_pwd,_ := path.Split(_file)
		os.Chdir(_pwd)
		pwd, _ := os.Getwd()
		queryPool = make(chan tIp2LocationReq, queryLength)
		recodePool = make(chan tIp2LocationResp, queryLength)
		filename.WriteString(pwd);
		filename.WriteString("/");
		filename.WriteString("qqwry.dat");
	}else{
		filename.WriteString(*dbfile);
	}
	log.Println("Load Data File:%v",filename.String());
	lib.Daemon(0, 1)

	//启动查询进程
	go startQueryService(filename.String())

	srv := tIp2LocationServiceJSONP{}
	http.Handle("/", &srv)
	err := http.ListenAndServe(*bindaddr, nil)
	if err != nil {
		log.Println("Error:");
		log.Println(err)
		log.Fatal("Exit!!!!!!!!!!!!!!!!!!!!!!!!!!!!");
	}

}
