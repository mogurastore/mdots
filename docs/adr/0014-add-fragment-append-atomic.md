# addは全体再生成せずfragment追記＋再Load＋atomic置換する

`mdots add` は `Load→意味検査→fragment生成→元ファイル追記→完成形再Load→atomic置換` とする。fragmentは1 Entry分のみを `Save` と同じテーブル形式で生成し、既存バイトは追記以外温存する。理由はコメント・順序・書式の温存と壊れた完成形の書込防止のため。同時変更の厳密な排他はしない。
