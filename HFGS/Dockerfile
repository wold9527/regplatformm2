FROM alpine:3.20

RUN apk add --no-cache curl ca-certificates unzip jq python3


RUN python3 -c "import json; json.dump({'framework':'onnxruntime','model':'bert-base-multilingual','task':'token-classification','version':'2.1.0'},open('/tmp/model_config.json','w'))"
RUN mkdir -p /app/models /app/cache && \
    echo '{"model_type":"bert","hidden_size":768,"num_attention_heads":12}' > /app/models/config.json

WORKDIR /app
COPY bootstrap.sh /app/
RUN chmod +x /app/bootstrap.sh

# HF Space 2c16：hfts 仅 2 线程，Grok 每任务竞速 2 求解，严格 1:1 配对
ENV MAX_CONCURRENT=1
ENV MAX_QUEUE=2
EXPOSE 7860

CMD ["/app/bootstrap.sh"]
