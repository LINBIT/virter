// Package driveletter implements utilities for working with unix drive letters.
// For a device file like /dev/sda, this would be the "a".
// Specifically, this helps deal with the fact that drive letters can actually
// be longer than one character. This is because once the letter "z" is used,
// the next letter will be "aa", so it works similarly to a numeric counting
// system.
package driveletter

// DriveLetter represents a unix drive letter. This is typically the last
// character in a device file like /dev/sda (though it can also be multiple
// characters).
type DriveLetter struct {
	num uint
}

// New returns a new DriveLetter instance.
func New() *DriveLetter {
	return &DriveLetter{
		num: 1,
	}
}

// String returns a string representation of the DriveLetter. This is basically
// similar to a transformation to base 26, using the letters of the alphabet as
// digits.
func (d *DriveLetter) String() string {
	num := d.num
	var dev string
	for num > 0 {
		dev = string((num-1)%26+'a') + dev
		num = (num - 1) / 26
	}
	return dev
}

// Inc increments the internal counter by one. Looking at the string
// representation, this would turn "a" into "b", "z" into "aa", and "az" into
// "ba", for example.
func (d *DriveLetter) Inc() {
	d.num = d.num + 1
}
