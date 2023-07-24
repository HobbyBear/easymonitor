

大家好,我是蓝胖子，关于性能分析的视频和文章我也大大小小出了有一二十篇了，算是已经有了一个系列，之前的代码已经上传到github.com/HobbyBear/performance-analyze，接下来这段时间我将在之前内容的基础上，结合自己在公司生产上构建监控系统的经验，详细的展示如何对线上服务进行监控，内容涉及到的指标设计，软件配置，监控方案等等你都可以拿来直接复刻到你的项目里，这是一套非常适合中小企业的监控体系。


## 监控系统架构

![image.png](https://s2.loli.net/2023/07/24/sApQNvodkEaW6Jx.png)

## 目录结构

```shell
(base) ➜  easymonitor git:(main) ✗ tree -L 1
.
├── ReadMe.md
├── build.sh // 对webhookserver 以及 webapp 项目进行编译 ，然后放到program文件夹里
├── docker-compose.yaml // 启动各个监控系统组件
├── filebeat.yml // filebeat日志采集的配置文件
├── go.mod
├── go.sum
├── grafanadashbord // 放置grafana的监控面板导出的json文件，可直接启动项目，然后导入json文件，即可构建监控面板
├── infra // 项目基础组件的代码，因为服务的监控有时会涉及到埋点和prometheus client暴露指标，将这部分逻辑都写在这个包下，后续新应用只要引入这个包就能拥有这些监控指标
├── logconf // 放置主机上的日志采集配置文件，filebeat.yml 中会引入这个文件夹下的配置规则做不同的采集策略
├── logs // 放置应用服务日志的目录，由于是docker-compose启动，需要将主机这个目录同时映射到filebeat和应用服务容器，filebeat会对这个目录下的日志进行采集
├── logstash.conf // logstash 配置文件
├── program // 放置webhookserver 以及 webapp 项目编译好的二进制文件
├── prometheus.yml // prometheus 配置文件
├── webapp // 应用服务代码
└── alerterserver // 模拟自研报警系统代码
```

## 启动步骤

```shell
cd easymonitor
sh build.sh 
docker-compose up 
```

