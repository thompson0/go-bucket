package draw

import(
	"fmt"
	"github.com/schollz/progressbar/v3"
   "time"
)

func BucketDraw(){
	fmt.Println(`


 _____ ____        ____  _     ____  _  __ _____ _____
/  __//  _ \      /  _ \/ \ /\/   _\/ |/ //  __//__ __\ 
| |  _| / \|_____ | | //| | |||  /  |   / |  \    / \    
| |_//| \_/|\____\| |_\\| \_/||  \_ |   \ |  /_   | |                
\____\\____/      \____/\____/\____/\_|\_\\____\  \_/
                                                        
----------------------------------------------------------------------------------------------------------
© [2026] [thompson0]
----------------------------------------------------------------------------------------------------------
	
`)
}
func StartProgessbar(total int) {
   if total <= 0 {
      total = 100
   }

   bar := progressbar.Default(int64(total))
   for atribute := 0; atribute < total; atribute++ {
      _ = bar.Add(1)
      time.Sleep(40 * time.Millisecond)
   }
}