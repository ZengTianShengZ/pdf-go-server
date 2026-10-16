# pdf-go-server

Single-container HTML → PDF service. Go API injects request JSON into a local HTML template and converts it with [Gotenberg](https://github.com/gotenberg/gotenberg) (Chromium) running on localhost inside the same container.

## Quick start

```bash
docker build -t pdf-go-server .
docker run --rm -p 8080:80 pdf-go-server
```

Generate a PDF:

```bash
curl -X POST http://localhost:8080/api/pdf \
  -H 'content-type: application/json' \
  -d '{
    "name": "John Doe",
    "email": "john.doe@example.com",
    "phone": "1234567890",
    "address": "123 Main St, Anytown, USA",
    "city": "Anytown",
    "state": "CA",
    "zip": "12345",
    "country": "USA"
  }' \
  -o report.pdf
```

Health check:

```bash
curl http://localhost:8080/health
```

## API

### `POST /api/pdf`

- Body: JSON object (template data)
- Success: `200` `application/pdf`
- Errors: JSON `{ "code": <http status>, "message": "..." }`

### `GET /health`

- Gotenberg reachable: `200` `{ "status": "ok" }`
- Otherwise: `503` `{ "status": "unavailable", "message": "gotenberg unreachable" }`

## Template

Uses `template/v1-home-inspection.html`. The service replaces the object literal assigned to `window.__REPORT_DATA__` with the request JSON; Chromium runs the page JS before PDF capture.

## Environment

| Variable | Default | Description |
|---|---|---|
| `PORT` | `80` | Go listen port |
| `GOTENBERG_URL` | `http://127.0.0.1:3000` | In-container Gotenberg |
| `TEMPLATE_PATH` | `template/v1-home-inspection.html` | HTML template path |
| `PDF_TIMEOUT_SEC` | `60` | Conversion timeout |

## Local unit tests

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.22-alpine go test ./...
```



docker build -t pdf-go-server .
docker run --rm -p 8080:80 pdf-go-server


curl -X POST http://localhost:8080/api/pdf \
  -H 'content-type: application/json' \
  -d '{"name":"John Doe","email":"john.doe@example.com","phone":"123","address":"123 Main St","city":"Anytown","state":"CA","zip":"12345","country":"USA"}' \
  -o report.3pdf




curl -X POST http://localhost:8080/api/pdf \
  -H 'content-type: application/json' \
  -d '{"id":"mock_insp_20260316_001","status":"completed","pdfInfo":{"bannerUrl":"https://mp-9a185928-f19d-4718-922b-bc72f8dca136.cdn.bspapp.com/images/ScreenShot_2026-10-15_180935_435.png"},"baseInfo":{"estateName":"绿城桂语江南","unitNo":"3栋2单元1802","area":"128.5","layout":"三室两厅两卫","houseType":"hardcover","inspectDate":"2026-03-16","ownerName":"张先生","ownerPhone":"13800138000"},"checklist":[{"id":"r_mock_entry","name":"入户门","issues":[{"id":"i_mock_entry_1","description":"门锁开启不顺畅，钥匙插入后需反复拨动才能开锁","suggestion":"联系物业或开发商调整锁芯/锁体，必要时更换五金配件","severity":"moderate","images":[{"id":"img_mock_entry_1a","url":"https://picsum.photos/seed/insp-entry-lock/800/600"}]},{"id":"i_mock_entry_2","description":"门扇与门框缝隙不均匀，关闭后下部露缝约 3mm","suggestion":"调整合页铰链或门扇垂直度，保证四周缝隙均匀","severity":"minor","images":[]}]},{"id":"r_mock_living","name":"客厅","issues":[{"id":"i_mock_living_1","description":"客厅南墙涂料局部有细微开裂，靠近踢脚线处约 20cm 裂纹","suggestion":"铲除开裂涂层后重新批腻子、打磨并补刷同色涂料","severity":"minor","images":[{"id":"img_mock_living_1a","url":"https://picsum.photos/seed/insp-living-crack/800/600"},{"id":"img_mock_living_1b","url":"https://picsum.photos/seed/insp-living-crack2/800/600"}]},{"id":"i_mock_living_2","description":"木地板局部空鼓，踩踏有明显异响（茶几下方区域）","suggestion":"切开空鼓区域重新铺贴或注胶加固，复测平整度","severity":"moderate","images":[{"id":"img_mock_living_2a","url":"https://picsum.photos/seed/insp-living-floor/800/600"}]},{"id":"i_mock_living_3","description":"客厅开关面板松动，按压时面板晃动","suggestion":"紧固底盒螺丝并校准面板安装","severity":"minor","images":[]}]},{"id":"r_mock_kitchen","name":"厨房","issues":[{"id":"i_mock_kitchen_1","description":"油烟机烟管与公共烟道接口密封不严，开机有轻微倒烟","suggestion":"重新做烟管接口密封，检查止回阀是否完好","severity":"severe","images":[{"id":"img_mock_kitchen_1a","url":"https://picsum.photos/seed/insp-kitchen-hood/800/600"},{"id":"img_mock_kitchen_1b","url":"https://picsum.photos/seed/insp-kitchen-duct/800/600"}]},{"id":"i_mock_kitchen_2","description":"橱柜台面与墙面交接处美缝不饱满，存在渗水隐患","suggestion":"清理后重新打密封胶或美缝，确保防水连续","severity":"moderate","images":[{"id":"img_mock_kitchen_2a","url":"https://picsum.photos/seed/insp-kitchen-counter/800/600"}]},{"id":"i_mock_kitchen_3","description":"地漏排水速度偏慢，蓄水约 2cm 时排出较缓","suggestion":"疏通下水管道并检查地漏箅子是否堵塞","severity":"minor","images":[]}]},{"id":"r_mock_mbath","name":"主卫","issues":[{"id":"i_mock_mbath_1","description":"主卫闭水试验 24 小时后，楼下对应位置天花板出现渗水痕迹","suggestion":"重做卫生间防水层并复测闭水，确认无渗漏后再恢复面层","severity":"severe","images":[{"id":"img_mock_mbath_1a","url":"https://picsum.photos/seed/insp-mbath-leak/800/600"},{"id":"img_mock_mbath_1b","url":"https://picsum.photos/seed/insp-mbath-ceiling/800/600"},{"id":"img_mock_mbath_1c","url":"https://picsum.photos/seed/insp-mbath-floor/800/600"}]},{"id":"i_mock_mbath_2","description":"马桶安装不稳，左右晃动，底座密封胶开裂","suggestion":"重新调平固定马桶，更换底座密封并补打密封胶","severity":"moderate","images":[{"id":"img_mock_mbath_2a","url":"https://picsum.photos/seed/insp-mbath-toilet/800/600"}]}]},{"id":"r_mock_mbed","name":"主卧","issues":[{"id":"i_mock_mbed_1","description":"主卧窗户密封条局部脱落，关闭后有明显风感","suggestion":"更换破损密封条，检查窗扇五金锁点是否到位","severity":"moderate","images":[{"id":"img_mock_mbed_1a","url":"https://picsum.photos/seed/insp-mbed-window/800/600"}]},{"id":"i_mock_mbed_2","description":"衣柜柜扇关闭后缝隙不均，右侧柜门轻微下垂","suggestion":"调节柜门铰链高度与水平度","severity":"minor","images":[]}]},{"id":"r_mock_balcony","name":"阳台","issues":[{"id":"i_mock_balcony_1","description":"阳台地砖空鼓，敲击有空洞声（靠近排水口区域约 0.3㎡）","suggestion":"铲除空鼓地砖后重新铺贴，并检查排水坡度","severity":"moderate","images":[{"id":"img_mock_balcony_1a","url":"https://picsum.photos/seed/insp-balcony-tile/800/600"}]},{"id":"i_mock_balcony_2","description":"护栏法兰盖缺失 1 处，基座螺丝外露","suggestion":"补装法兰盖并紧固基座螺丝，做好防锈处理","severity":"minor","images":[]},{"id":"i_mock_balcony_3","description":"墙体水分检测偏高（阳台与客厅交接墙面），读数超过正常范围","suggestion":"排查外墙/窗边渗水点，干燥后复测并视情况处理防水","severity":"severe","images":[{"id":"img_mock_balcony_3a","url":"https://picsum.photos/seed/insp-balcony-moisture/800/600"},{"id":"img_mock_balcony_3b","url":"https://picsum.photos/seed/insp-balcony-wall/800/600"}]}]}],"createTime":1742054400000,"updateTime":1742140800000}' \
  -o report4.pdf