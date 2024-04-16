# gcron

这是一个周期性任务库，代码很大程度上参考了 [cron](https://github.com/robfig/cron)，主要添加了任务持久化的功能；
默认把任务保存到 cron.json 文件中，可以自定义保存到其他地方。

具体用法请参考 [cron_test.go](./test/cron_test.go)，仍然存在着很多问题，欢迎提 PR 或者 Issues。