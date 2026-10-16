# 本地 / 云托管调用示例

```bash
docker build -t pdf-go-server .
docker run --rm -p 8080:80 pdf-go-server
```

## 推荐：用 JSON 文件作 body（避免换行把 JSON 弄坏）

本地：

```bash
curl -X POST http://localhost:8080/api/pdf \
  -H 'content-type: application/json' \
  --data-binary @template/mock-inspection.json \
  -o report.pdf
```

云托管：

```bash
curl -X POST https://golang-bdzs-329598-8-1351965298.sh.run.tcloudbase.com/api/pdf \
  -H 'content-type: application/json' \
  --data-binary @template/mock-inspection.json \
  -o report.pdf
```

成功后应看到体积明显大于 1KB，且：

```bash
file report.pdf
# PDF document, version 1.4, ...
```

若 `file` 显示 `ASCII text` / `JSON data`，说明不是 PDF。直接看内容：

```bash
cat report.pdf
# 常见错误：
# 404 page not found          → URL/路径写错（少了 /api/pdf）
# {"code":400,"message":"..."} → JSON 不合法
```

## 微信云托管 / 小程序

请用 `?format=base64`，见 README「微信云托管注意」。
