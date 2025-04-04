This command works to build without using microsoft visual studio community edition

```
wix build -pdbtype none -arch x64 -ext "C:\Users\erikr\.nuget\packages\wixtoolset.util.wixext\5.0.0\wixext5\WixToolset.Util.wixext.dll" -ext "C:\Users\erikr\.nuget\packages\wixtoolset.ui.wixext\5.0.0\wixext5\WixToolset.UI.wixext.dll" -o "C:\workspace\alloy\dist\alloy-installer-windows-x64.msi" Product.wxs UI.wxs AlloyFiles.wxs AlloyRegistry.wxs AlloyService.wxs
```