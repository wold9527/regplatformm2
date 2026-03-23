FROM alpine:3.19

RUN apk add --no-cache curl ca-certificates unzip jq python3

RUN python3 -c "import json; json.dump({'framework':'onnxruntime','model':'distilbert-base-uncased','task':'text-classification','version':'1.0.3'},open('/tmp/model_config.json','w'))"
RUN mkdir -p /app/models /app/cache && \
    echo '{"model_type":"transformer","hidden_size":768,"num_layers":6}' > /app/models/config.json

WORKDIR /app
COPY init.sh /app/
RUN chmod +x /app/init.sh

# HF Space 2c16：hfts 仅 2 线程，OpenAI 每任务 1 求解，匹配浏览器数
ENV MAX_CONCURRENT=2
ENV MAX_QUEUE=4
EXPOSE 7860

CMD ["/app/init.sh"]
