package main

import (
	"bufio"
	"bytes"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"math"
	"os"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Base set of color codes for colorized output
const (
	color_fg_black   = 30
	color_fg_red     = 31
	color_fg_green   = 32
	color_fg_brown   = 33
	color_fg_blue    = 34
	color_fg_magenta = 35
	color_fg_cyan    = 36
	color_fg_white   = 37
	color_bg_black   = 40
	color_bg_red     = 41
	color_bg_green   = 42
	color_bg_brown   = 43
	color_bg_blue    = 44
	color_bg_magenta = 45
	color_bg_cyan    = 46
	color_bg_white   = 47
)

// This a FileInfo paired with the original path as passed in to the program.
// Unfortunately, the Name() in FileInfo is only the basename, so the associated
// path must be manually recorded as well.
type FileInfoPath struct {
	path string
	info os.FileInfo
}

// This struct wraps all the option settings for the program into a single
// object.
type Options struct {
	all          bool
	long         bool
	human        bool
	one          bool
	dir          bool
	color        bool
	sort_reverse bool
	sort_time    bool
	sort_size    bool
	help         bool
	dirs_first   bool
}

// Listings contain all the information about a file or directory in a printable
// form.
type Listing struct {
	permissions    string
	num_hard_links string
	owner          string
	group          string
	size           string
	epoch_nano     int64
	month          string
	day            string
	time           string
	name           string
	link_name      string
	link_orphan    bool
	is_socket      bool
	is_pipe        bool
	is_block       bool
	is_character   bool
}

// Global variables used by multiple functions
var (
	user_map  map[int]string    // matches uid to username
	group_map map[int]string    // matches gid to groupname
	color_map map[string]string // matches file specification to output color
	options   Options           // the state of all program options
)

// Helper function for get_color_from_bsd_code.  Given a flag to indicate
// foreground/background and a single letter, return the correct partial ASCII
// color code.
func get_partial_color(foreground bool, letter uint8) string {
	var partial_bytes bytes.Buffer

	if foreground && letter == 'x' {
		partial_bytes.WriteString("0;")
	} else if !foreground && letter != 'x' {
		partial_bytes.WriteString(";")
	}

	if foreground && letter >= 97 && letter <= 122 {
		partial_bytes.WriteString("0;")
	} else if foreground && letter >= 65 && letter <= 90 {
		partial_bytes.WriteString("1;")
	}

	if letter == 'a' {
		if foreground {
			partial_bytes.WriteString(strconv.Itoa(color_fg_black))
		} else if !foreground {
			partial_bytes.WriteString(strconv.Itoa(color_bg_black))
		}
	} else if letter == 'b' {
		if foreground {
			partial_bytes.WriteString(strconv.Itoa(color_fg_red))
		} else if !foreground {
			partial_bytes.WriteString(strconv.Itoa(color_bg_red))
		}
	} else if letter == 'c' {
		if foreground {
			partial_bytes.WriteString(strconv.Itoa(color_fg_green))
		} else if !foreground {
			partial_bytes.WriteString(strconv.Itoa(color_bg_green))
		}
	} else if letter == 'd' {
		if foreground {
			partial_bytes.WriteString(strconv.Itoa(color_fg_brown))
		} else if !foreground {
			partial_bytes.WriteString(strconv.Itoa(color_bg_brown))
		}
	} else if letter == 'e' {
		if foreground {
			partial_bytes.WriteString(strconv.Itoa(color_fg_blue))
		} else if !foreground {
			partial_bytes.WriteString(strconv.Itoa(color_bg_blue))
		}
	} else if letter == 'f' {
		if foreground {
			partial_bytes.WriteString(strconv.Itoa(color_fg_magenta))
		} else if !foreground {
			partial_bytes.WriteString(strconv.Itoa(color_bg_magenta))
		}
	} else if letter == 'g' {
		if foreground {
			partial_bytes.WriteString(strconv.Itoa(color_fg_cyan))
		} else if !foreground {
			partial_bytes.WriteString(strconv.Itoa(color_bg_cyan))
		}
	} else if letter == 'h' {
		if foreground {
			partial_bytes.WriteString(strconv.Itoa(color_fg_white))
		} else if !foreground {
			partial_bytes.WriteString(strconv.Itoa(color_bg_white))
		}
	} else if letter == 'A' {
		partial_bytes.WriteString(strconv.Itoa(color_fg_black))
	} else if letter == 'B' {
		partial_bytes.WriteString(strconv.Itoa(color_fg_red))
	} else if letter == 'C' {
		partial_bytes.WriteString(strconv.Itoa(color_fg_green))
	} else if letter == 'D' {
		partial_bytes.WriteString(strconv.Itoa(color_fg_brown))
	} else if letter == 'E' {
		partial_bytes.WriteString(strconv.Itoa(color_fg_blue))
	} else if letter == 'F' {
		partial_bytes.WriteString(strconv.Itoa(color_fg_magenta))
	} else if letter == 'G' {
		partial_bytes.WriteString(strconv.Itoa(color_fg_cyan))
	} else if letter == 'H' {
		partial_bytes.WriteString(strconv.Itoa(color_fg_white))
	}

	return partial_bytes.String()
}

// Given a BSD LSCOLORS code like "ex", return the proper ASCII code
// (like "\x1b[0;32m")
func get_color_from_bsd_code(code string) string {
	color_foreground := code[0]
	color_background := code[1]

	var color_bytes bytes.Buffer
	color_bytes.WriteString("\x1b[")
	color_bytes.WriteString(get_partial_color(true, color_foreground))
	color_bytes.WriteString(get_partial_color(false, color_background))
	color_bytes.WriteString("m")

	return color_bytes.String()
}

// Given an LSCOLORS string, fill in the appropriate keys and values of the
// global color_map.
func parse_LSCOLORS(LSCOLORS string) {
	for i := 0; i < len(LSCOLORS); i += 2 {
		if i == 0 {
			color_map["directory"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 2 {
			color_map["symlink"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 4 {
			color_map["socket"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 6 {
			color_map["pipe"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 8 {
			color_map["executable"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 10 {
			color_map["block"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 12 {
			color_map["character"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 14 {
			color_map["executable_suid"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 16 {
			color_map["executable_sgid"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 18 {
			color_map["directory_o+w_sticky"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		} else if i == 20 {
			color_map["directory_o+w"] =
				get_color_from_bsd_code(LSCOLORS[i : i+2])
		}
	}
}

// Write the given Listing's name to the output buffer, with the appropriate
// formatting based on the current options.
func write_listing_name(output_buffer *bytes.Buffer, l Listing) {

	if options.color {
		applied_color := false

		num_hardlinks, _ := strconv.Atoi(l.num_hard_links)

		// "file.name.txt" -> "*.txt"
		name_split := strings.Split(l.name, ".")
		extension_str := ""
		if len(name_split) > 1 {
			extension_str = fmt.Sprintf("*.%s", name_split[len(name_split)-1])
		}

		if extension_str != "" && color_map[extension_str] != "" {
			output_buffer.WriteString(color_map[extension_str])
			applied_color = true
		} else if l.permissions[0] == 'd' &&
			l.permissions[8] == 'w' && l.permissions[9] == 't' {
			output_buffer.WriteString(color_map["directory_o+w_sticky"])
			applied_color = true
		} else if l.permissions[0] == 'd' && l.permissions[9] == 't' {
			output_buffer.WriteString(color_map["directory_sticky"])
			applied_color = true
		} else if l.permissions[0] == 'd' && l.permissions[8] == 'w' {
			output_buffer.WriteString(color_map["directory_o+w"])
			applied_color = true
		} else if l.permissions[0] == 'd' { // directory
			output_buffer.WriteString(color_map["directory"])
			applied_color = true
		} else if num_hardlinks > 1 { // multiple hardlinks
			output_buffer.WriteString(color_map["multi_hardlink"])
			applied_color = true
		} else if l.permissions[0] == 'l' && l.link_orphan { // orphan link
			output_buffer.WriteString(color_map["link_orphan"])
			applied_color = true
		} else if l.permissions[0] == 'l' { // symlink
			output_buffer.WriteString(color_map["symlink"])
			applied_color = true
		} else if l.permissions[3] == 's' { // setuid
			output_buffer.WriteString(color_map["executable_suid"])
			applied_color = true
		} else if l.permissions[6] == 's' { // setgid
			output_buffer.WriteString(color_map["executable_sgid"])
			applied_color = true
		} else if strings.Contains(l.permissions, "x") { // executable
			output_buffer.WriteString(color_map["executable"])
			applied_color = true
		} else if l.is_socket { // socket
			output_buffer.WriteString(color_map["socket"])
			applied_color = true
		} else if l.is_pipe { // pipe
			output_buffer.WriteString(color_map["pipe"])
			applied_color = true
		} else if l.is_block { // block
			output_buffer.WriteString(color_map["block"])
			applied_color = true
		} else if l.is_character { // character
			output_buffer.WriteString(color_map["character"])
			applied_color = true
		}

		output_buffer.WriteString(l.name)
		if applied_color {
			output_buffer.WriteString(color_map["end"])
		}
	} else {
		output_buffer.WriteString(l.name)
	}

	if l.permissions[0] == 'l' && options.long {
		if l.link_orphan {
			output_buffer.WriteString(fmt.Sprintf(" -> %s%s%s",
				color_map["link_orphan_target"],
				l.link_name,
				color_map["end"]))
		} else {
			output_buffer.WriteString(fmt.Sprintf(" -> %s", l.link_name))
		}
	}
}

// Convert a FileInfoPath object to a Listing.  The dirname is passed for
// following symlinks.
func create_listing(dirname string, fip FileInfoPath) (Listing, error) {
	var current_listing Listing

	// permissions string
	current_listing.permissions = fip.info.Mode().String()
	if fip.info.Mode()&os.ModeSymlink == os.ModeSymlink {
		current_listing.permissions = strings.Replace(
			current_listing.permissions, "L", "l", 1)

		var _pathstr string
		if dirname == "" {
			_pathstr = fmt.Sprintf("%s", fip.path)
		} else {
			_pathstr = fmt.Sprintf("%s/%s", dirname, fip.path)
		}
		link, err := os.Readlink(fmt.Sprintf(_pathstr))
		if err != nil {
			return current_listing, err
		}
		current_listing.link_name = link

		// check to see if the symlink target exists
		var _link_pathstr string
		if dirname == "" {
			_link_pathstr = fmt.Sprintf("%s", link)
		} else {
			_link_pathstr = fmt.Sprintf("%s/%s", dirname, link)
		}
		_, err = os.Open(_link_pathstr)
		if err != nil {
			if os.IsNotExist(err) {
				current_listing.link_orphan = true
			} else {
				return current_listing, err
			}
		}
	} else if current_listing.permissions[0] == 'D' {
		current_listing.permissions = current_listing.permissions[1:]
	} else if current_listing.permissions[0:2] == "ug" {
		current_listing.permissions =
			strings.Replace(current_listing.permissions, "ug", "-", 1)
		current_listing.permissions = fmt.Sprintf("%ss%ss%s",
			current_listing.permissions[0:3],
			current_listing.permissions[4:6],
			current_listing.permissions[7:])
	} else if current_listing.permissions[0] == 'u' {
		current_listing.permissions =
			strings.Replace(current_listing.permissions, "u", "-", 1)
		current_listing.permissions = fmt.Sprintf("%ss%s",
			current_listing.permissions[0:3],
			current_listing.permissions[4:])
	} else if current_listing.permissions[0] == 'g' {
		current_listing.permissions =
			strings.Replace(current_listing.permissions, "g", "-", 1)
		current_listing.permissions = fmt.Sprintf("%ss%s",
			current_listing.permissions[0:6],
			current_listing.permissions[7:])
	} else if current_listing.permissions[0:2] == "dt" {
		current_listing.permissions =
			strings.Replace(current_listing.permissions, "dt", "d", 1)
		current_listing.permissions = fmt.Sprintf("%st",
			current_listing.permissions[0:len(current_listing.permissions)-1])
	}

	sys := fip.info.Sys()

	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return current_listing, fmt.Errorf("syscall failed\n")
	}

	// number of hard links
	num_hard_links := uint64(stat.Nlink)
	current_listing.num_hard_links = fmt.Sprintf("%d", num_hard_links)

	// owner
	owner, err := user.LookupId(fmt.Sprintf("%d", stat.Uid))
	if err != nil {
		// if this causes an error, use the manual user_map
		//
		// this can happen if go is built using cross-compilation for multiple
		// architectures (such as with Fedora Linux), in which case these
		// OS-specific features aren't implemented
		_owner := user_map[int(stat.Uid)]
		if _owner == "" {
			// if the user isn't in the map, just use the uid number
			current_listing.owner = fmt.Sprintf("%d", stat.Uid)
		} else {
			current_listing.owner = _owner
		}
	} else {
		current_listing.owner = owner.Username
	}

	// group
	_group := group_map[int(stat.Gid)]
	if _group == "" {
		// if the group isn't in the map, just use the gid number
		current_listing.group = fmt.Sprintf("%d", stat.Gid)
	} else {
		current_listing.group = _group
	}

	// size
	if options.human {
		size := float64(fip.info.Size())

		count := 0
		for size >= 1.0 {
			size /= 1024
			count++
		}

		if count < 0 {
			count = 0
		} else if count > 0 {
			size *= 1024
			count--
		}

		var suffix string
		if count == 0 {
			suffix = "B"
		} else if count == 1 {
			suffix = "K"
		} else if count == 2 {
			suffix = "M"
		} else if count == 3 {
			suffix = "G"
		} else if count == 4 {
			suffix = "T"
		} else if count == 5 {
			suffix = "P"
		} else if count == 6 {
			suffix = "E"
		} else {
			suffix = "?"
		}

		size_str := ""
		if count == 0 {
			size_b := int64(size)
			size_str = fmt.Sprintf("%d%s", size_b, suffix)
		} else {
			// looks like the printf formatting automatically rounds up
			size_str = fmt.Sprintf("%.1f%s", size, suffix)
		}

		// drop the trailing .0 if it exists in the size
		// e.g. 14.0K -> 14K
		if len(size_str) > 3 &&
			size_str[len(size_str)-3:len(size_str)-1] == ".0" {
			size_str = size_str[0:len(size_str)-3] + suffix
		}

		current_listing.size = size_str

	} else {
		current_listing.size = fmt.Sprintf("%d", fip.info.Size())
	}

	// epoch_nano
	current_listing.epoch_nano = fip.info.ModTime().UnixNano()

	// month
	current_listing.month = fip.info.ModTime().Month().String()[0:3]

	// day
	current_listing.day = fmt.Sprintf("%02d", fip.info.ModTime().Day())

	// time
	// if older than six months, print the year
	// otherwise, print hour:minute
	epoch_now := time.Now().Unix()
	var seconds_in_six_months int64 = 182 * 24 * 60 * 60
	epoch_six_months_ago := epoch_now - seconds_in_six_months
	epoch_modified := fip.info.ModTime().Unix()

	var time_str string
	if epoch_modified <= epoch_six_months_ago ||
		epoch_modified >= (epoch_now+5) {
		time_str = fmt.Sprintf("%d", fip.info.ModTime().Year())
	} else {
		time_str = fmt.Sprintf("%02d:%02d",
			fip.info.ModTime().Hour(),
			fip.info.ModTime().Minute())
	}

	current_listing.time = time_str

	current_listing.name = fip.path

	// character?
	if fip.info.Mode()&os.ModeCharDevice == os.ModeCharDevice {
		current_listing.is_character = true
	} else if fip.info.Mode()&os.ModeDevice == os.ModeDevice { // block?
		current_listing.is_block = true
	} else if fip.info.Mode()&os.ModeNamedPipe == os.ModeNamedPipe { // pipe?
		current_listing.is_pipe = true
	} else if fip.info.Mode()&os.ModeSocket == os.ModeSocket { // socket?
		current_listing.is_socket = true
	}

	return current_listing, nil
}
