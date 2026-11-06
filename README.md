# kakaotalk-linux

리눅스에서 카카오톡을 쉽게 설치하고, 데스크톱에 어울리게 쓰도록 도와주는 **비공식** 도구예요.
카카오와 관계없는 개인 프로젝트이고, 카카오톡 프로그램을 수정하거나 함께 배포하지 않아요.
설치할 때 카카오 공식 설치 파일을 내려받아 사용해요.

> **아직 초기 버전이에요.** Fedora 44 GNOME에서만 시험했어요. 다른 환경의 결과를 이슈로 알려 주시면 큰 도움이 돼요.

## 설치

```bash
git clone https://github.com/seolcu/kakaotalk-linux.git
cd kakaotalk-linux
./install.sh
```

관리자 권한은 필요 없고, 홈 폴더에만 설치돼요. 내려받는 양이 많아서(약 1.4GB) 처음에는 몇 분 걸려요.
설치가 끝나면 앱 메뉴에서 **카카오톡**을 실행하세요.

필요한 것: x86_64 PC, `python3`, `curl`, `tar`, 한글 글꼴

**GNOME 사용자:** 트레이 아이콘을 보려면
[AppIndicator 확장](https://extensions.gnome.org/extension/615/appindicator-support/)이 필요해요.
Ubuntu에는 기본으로 들어 있어요.

## 무엇이 다른가요

| 기능 | 내용 |
|---|---|
| **트레이** | 데스크톱이 직접 그리는 메뉴를 써요. 왼쪽 클릭으로 창을 열고, 오른쪽 클릭으로 메뉴를 열어요. |
| **파일 선택창** | 첨부와 저장에 데스크톱의 파일 선택창을 써요. |
| **느린 로그인 방지** | 공용 Wi-Fi에서 Wine에 쌓인 오디오 장치 목록을 실행 전에 정리해요. |
| **Steam 불필요** | [umu-launcher](https://github.com/Open-Wine-Components/umu-launcher)와 GE-Proton으로 실행해요. |

## 알려진 문제

- 😀 같은 이모지가 네모로 보여요. Wine의 글자 그리기 한계예요.
- 한글 입력 중 다음 글자의 초성이 모음을 칠 때까지 안 보일 수 있어요. 입력 결과는 정상이에요.
- 카카오톡이 직접 그리는 본문 창과 알림은 시스템 스타일로 바꿀 수 없어요.
- 카카오톡 업데이트 후 트레이 메뉴 일부가 동작하지 않을 수 있어요. 그러면 이슈로 알려 주세요.
- 카카오톡은 PC 한 대에서만 로그인돼요. 새로 로그인하면 다른 PC의 세션이 끝날 수 있어요.

## 문제 해결

터미널에서 `kakaotalk`을 실행하면 오류 메시지를 볼 수 있어요. 이슈를 올릴 때 배포판, 데스크톱 환경과 함께 적어 주세요.

공용 Wi-Fi에서 다른 사람의 Mac이 소리 출력 장치로 뜬다면 PipeWire의 AirPlay 자동 발견 때문이에요.
쓰지 않는다면 `~/.config/pipewire/pipewire.conf.d/90-disable-raop.conf`에 아래 내용을 넣고 다시 로그인하세요.

```
context.properties = { module.raop = false }
```

## 삭제

```bash
./uninstall.sh
```

이 PC의 대화 데이터도 함께 지워져요.

## 라이선스

이 프로젝트의 코드는 MIT 라이선스예요. 카카오톡은 카카오의 소프트웨어이고 카카오톡 약관을 따라요.
