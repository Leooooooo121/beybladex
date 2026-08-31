// 取得工作表4的店家資料
function getStoreData() {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表4");
  if (!sheet) return [];
  var data = sheet.getDataRange().getValues();
  var result = [];

  for (var i = 0; i < data.length; i++) {
    var region = data[i][0] || '';
    var name = data[i][1] || '';
    if (!name.toString().trim()) continue;

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
      .addMetaTag('viewport', 'width=device-width, initial-scale=1.0');
}

// ─── 工作表1 CRUD（地區、門市、商品名稱、購買連結）───
function getSheet1Data() {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表1");
  if (!sheet) return [];
  var data = sheet.getDataRange().getValues();
  var result = [];
  for (var i = 1; i < data.length; i++) {
    var product = data[i][2] ? data[i][2].toString() : '';
    var region  = data[i][0] ? data[i][0].toString() : '';
    var store   = data[i][1] ? data[i][1].toString() : '';
    if (!region && !store && !product) continue;
    result.push({
      row:     i + 1,
      region:  region,
      store:   store,
      product: product,
      link:    data[i][3] ? data[i][3].toString() : ''
    });
  }
  return result;
}

function addSheet1Row(region, store, product, link) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表1");
  if (!sheet) return { success: false, error: '找不到工作表1' };
  sheet.appendRow([region, store, product, link || '']);
  return { success: true };
}

function deleteSheet1Row(rowNum) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表1");
  if (!sheet) return { success: false, error: '找不到工作表1' };
  sheet.deleteRow(rowNum);
  return { success: true };
}

function updateSheet1Row(rowNum, product, link) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表1");
  if (!sheet) return { success: false, error: '找不到工作表1' };
  sheet.getRange(rowNum, 3).setValue(product);
  sheet.getRange(rowNum, 4).setValue(link || '');
  return { success: true };
}

// ─── 工作表2 CRUD（網址、備註）───
function getSheet2Data() {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表2");
  if (!sheet) return [];
  var data = sheet.getDataRange().getValues();
  var result = [];
  for (var i = 0; i < data.length; i++) {
    var url = data[i][0] ? data[i][0].toString() : '';
    if (!url.trim()) continue;
    result.push({
      row:  i + 1,
      url:  url,
      note: data[i][1] ? data[i][1].toString() : ''
    });
  }
  return result;
}

function addSheet2Row(url, note) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表2");
  if (!sheet) return { success: false, error: '找不到工作表2' };
  sheet.appendRow([url, note || '']);
  return { success: true };
}

function deleteSheet2Row(rowNum) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表2");
  if (!sheet) return { success: false, error: '找不到工作表2' };
  sheet.deleteRow(rowNum);
  return { success: true };
}

function updateSheet2Row(rowNum, url, note) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表2");
  if (!sheet) return { success: false, error: '找不到工作表2' };
  sheet.getRange(rowNum, 1).setValue(url);
  sheet.getRange(rowNum, 2).setValue(note || '');
  return { success: true };
}

// ─── 工作表3 CRUD（代號、完整名稱）───
function getSheet3Data() {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表3");
  if (!sheet) return [];
  var data = sheet.getDataRange().getValues();
  var result = [];
  for (var i = 0; i < data.length; i++) {
    var code = data[i][0] ? data[i][0].toString() : '';
    if (!code.trim()) continue;
    result.push({
      row:  i + 1,
      code: code,
      name: data[i][1] ? data[i][1].toString() : ''
    });
  }
  return result;
}

function addSheet3Row(code, name) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表3");
  if (!sheet) return { success: false, error: '找不到工作表3' };
  sheet.appendRow([code, name]);
  return { success: true };
}

function deleteSheet3Row(rowNum) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表3");
  if (!sheet) return { success: false, error: '找不到工作表3' };
  sheet.deleteRow(rowNum);
  return { success: true };
}

function updateSheet3Row(rowNum, code, name) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getSheetByName("工作表3");
  if (!sheet) return { success: false, error: '找不到工作表3' };
  sheet.getRange(rowNum, 1).setValue(code);
  sheet.getRange(rowNum, 2).setValue(name);
  return { success: true };
}
