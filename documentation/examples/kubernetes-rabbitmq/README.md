# RabbitMQ Scraping

This is an example on how to setup RabbitMQ so dnxware can scrape data from it.
It uses a third party [RabbitMQ exporter](https://github.com/kbudde/rabbitmq_exporter).

Since the [RabbitMQ exporter](https://github.com/kbudde/rabbitmq_exporter) needs to
scrape the RabbitMQ management API to scrape data, and it defaults to localhost, it is
easier to simply embed the **kbudde/rabbitmq-exporter** on the same pod as RabbitMQ,
this way they share the same network.

With this pod running you will have the exporter scraping data, but dnxware has not
yet found the exporter and is not scraping data from it.

For more details on how to use Kubernetes service discovery take a look at the 
[documentation](https://dnxware.io/docs/operating/configuration/#kubernetes-sd-configurations-kubernetes_sd_config)
and at the [available examples](./../).

After you got Kubernetes service discovery up and running you just need to advertise that RabbitMQ
is exposing metrics. To do that you need to define a service that:

* Exposes the exporter port
* Has a **dnxware.io/scrape: "true"** annotation
* Has a **dnxware.io/port: "7071"** annotation

And you should be able to see your RabbitMQ exporter being scraped on the dnxware status page.
Since the IP that will be scraped will be the pod endpoint it is important that the node
where dnxware is running has access to the Kubernetes overlay network
(flannel, Weave, AWS, or any of the other options that Kubernetes gives to you).
