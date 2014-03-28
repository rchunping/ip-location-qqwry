ip2location-qqwry
=================

Golang做的ip城市查询服务，采用纯真数据库qqwry.dat



编译
----

~~~~
git clone https://github.com/rchunping/ip2location-qqwry.git
cd ip2location-qqwry
go build
~~~~


因为纯真数据是GBK编码的，这里自带了go-iconv转成UTF-8 ( https://github.com/ChaimHong/go-iconv  )

这个要cgo编译，如果编译失败，请自修修改 go-iconv/iconv.go 里面的

~~~~
// #cgo CFLAGS: -I/usr/local/include
// #cgo LDFLAGS: -liconv -L/usr/local/lib
// #include <iconv.h>
// #include <errno.h>
~~~~

基本上只需要调整CFLAGS和LDFLAGS


使用
----

~~~~
./ip2location-qqwry -h

Usage of ./ip2location-qqwry:
  -b="0.0.0.0:45356": listen port
  -f="qqwry.dat": database file
~~~~


源码中已经自带一个纯真ip数据库，大概是2014-03-20的版本

~~~~
./ip2location-qqwry -b ":45356" -f ./qqwry.dat
~~~~


调用方法
--------

~~~~
callback [可选]  jsonp回掉函数，如果不传，则返回json数据
ip       [可选]  查询的ip地址，如果不传，则自动检测
~~~~


成功返回

~~~~
$ curl   "127.0.0.1:45356/?callback=parse&ip=202.101.172.35"
parse({"area":"电信DNS服务器","country":"浙江省杭州市","ip":"202.101.172.35","ok":true})
~~~~

失败返回

~~~~
$ curl   "127.0.0.1:45356/?callback=parse&ip=213412341234"
parse({"area":"","country":"","ip":"213412341234","ok":false})
~~~~





性能
----

~~~~
$ ab -n 1000 -c 100  "127.0.0.1:45356/?callback=parse&ip=202.101.172.35"

....

Requests per second:    3690.54 [#/sec] (mean)
Time per request:       27.096 [ms] (mean)
Time per request:       0.271 [ms] (mean, across all concurrent requests)
Transfer rate:          792.89 [Kbytes/sec] received

Connection Times (ms)
              min  mean[+/-sd] median   max
Connect:        0    0   0.8      0       4
Processing:     3   25   5.4     27      55
Waiting:        3   25   5.4     27      55
Total:          6   26   5.0     27      55

....

~~~~

应该足够自己使用了。如果压力更大的坏境，可以把数据读入内存，还有些优化空间。


Nginx配置
---------

~~~~~
  location /ip2location {

      proxy_set_header X-Real-IP        $remote_addr;
      
      #proxy_set_header X-Forwarded-For  $proxy_add_x_forwarded_for;
      proxy_pass http://127.0.0.1:45356;
      proxy_redirect off;
  }
~~~~~

如果需要自动检测客户IP，需要 X-Real-Ip 或者 X-Forwarded-For 这个头


注意事项
--------

返回数据需要客户端做一些预处理，多数数据是 XX省XX市 的形式

国外数据不太准，需要另外的解决方案（购买完整版GeoIP或者免费缩水版）



