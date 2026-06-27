package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func cmdRemove(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: yd remove <文件路径...>")
		os.Exit(1)
	}

	ids := make([]string, 0, len(args))
	for _, p := range args {
		id, _, _, err := pathToID(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if id == RootFolder {
			fmt.Fprintln(os.Stderr, "不支持删除根目录")
			os.Exit(1)
		}
		ids = append(ids, id)
	}

	if err := doRemove(ids); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func doRemove(ids []string) error {
	params := make(url.Values)
	params.Set("fileIdList", strings.Join(ids, ";"))

	req, err := apiPost("/batchDeleteFile.action", params)
	if err != nil {
		return err
	}
	if err := doAPI(req, nil); err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	fmt.Printf("删除完成: %d 项\n", len(ids))
	return nil
}
