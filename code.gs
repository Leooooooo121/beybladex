// 取得工作表4的店家資料
function getStoreData() {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表4");
  if (!sheet) return [];
  var data = sheet.getDataRange().getValues();
  var result = [];

  for (var i = 0; i < data.length; i++) {
    var region = data[i][0] || '';
    var name = data[i][1] || '';
    if (!name.toString().trim()) continue; // 如果名稱為空則跳過

    result.push({
      region: region.toString(),
      name: name.toString(),
      voom: data[i][2] ? data[i][2].toString() : null,
      fb: data[i][3] ? data[i][3].toString() : null,
      line: data[i][4] ? data[i][4].toString() : null
    });
  }
  return result;
}

// 搭配側邊欄或內嵌網頁的進入點
function doGet() {
  return HtmlService.createHtmlOutputFromFile('Index')
      .setTitle('Funbox 店家資訊彙整')
      .addMetaTag('viewport', 'width=device-width, initial-scale=1.0'); // 關鍵：必須由後端加上這行
}